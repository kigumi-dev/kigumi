package driver_test

import (
	"fmt"
	"strings"
)

func validateCollateralVector(v *sgxVector, blobs map[string]string) error {
	need := func(names ...string) error {
		for _, name := range names {
			if _, ok := blobs[name]; !ok || name == "" {
				return fmt.Errorf("references missing blob %q", name)
			}
		}
		return nil
	}
	c := v.Collateral
	switch v.Op {
	case "sgx-verify":
		if err := need(v.Quote); err != nil {
			return err
		}
		if c != nil {
			return need(c.TcbInfo, c.TcbChain, c.QeIdentity, c.QeChain, c.PckCrl, c.RootCrl)
		}
	case "sgx-tcb":
		if c == nil || v.CpuSvn == "" {
			return fmt.Errorf("needs TCB Info and component SVNs")
		}
		return need(c.TcbInfo)
	case "sgx-qe-identity":
		if c == nil || v.Hex == "" {
			return fmt.Errorf("needs a QE Identity and a QE report")
		}
		return need(c.QeIdentity)
	case "sgx-collateral-auth":
		if c == nil || len(v.Args) != 1 || v.Anchors == "" {
			return fmt.Errorf("needs a document kind, collateral and a pinned root")
		}
		if v.Args[0] == "tcb" {
			return need(c.TcbInfo, c.TcbChain, c.RootCrl)
		}
		return need(c.QeIdentity, c.QeChain, c.RootCrl)
	}
	return nil
}

const sgxErrorArm = `Err(err) -> "error:${err.code.name()}"`

func emitCollateralVector(v sgxVector, blob func(string) string) (string, bool) {
	switch v.Op {
	case "sgx-verify":
		return emitVerify(v, blob), true
	case "sgx-tcb":
		return fmt.Sprintf(`match sgx.TcbInfo.parse(s(%s)) {
        Ok(info) -> match info.evaluate(b(%q), %d) {
            Ok(e) -> "ok status=${e.status.name()} date=${e.tcbDate} advisories=${joined(&e.advisoryIds)}"
            %s
        }
        %s
    }`, blob(v.Collateral.TcbInfo), v.CpuSvn, v.PceSvn, sgxErrorArm, sgxErrorArm), true
	case "sgx-qe-identity":
		return fmt.Sprintf(`match sgx.QeIdentity.parse(s(%s)) {
        Ok(identity) -> match sgx.Report.parse(b(%q)) {
            Ok(report) -> match identity.evaluate(&report) {
                Ok(e) -> "ok status=${e.status.name()} isvSvn=${e.isvSvn} date=${e.tcbDate} advisories=${joined(&e.advisoryIds)}"
                %s
            }
            %s
        }
        %s
    }`, blob(v.Collateral.QeIdentity), v.Hex, sgxErrorArm, sgxErrorArm, sgxErrorArm), true
	case "sgx-converge":
		return fmt.Sprintf(`match sgx.TcbStatus.parse(%q) {
        Ok(platform) -> match sgx.TcbStatus.parse(%q) {
            Ok(qe) -> match sgx.convergeStatus(platform, qe) {
                Ok(status) -> "ok ${status.name()}"
                %s
            }
            %s
        }
        %s
    }`, v.Args[0], v.Args[1], sgxErrorArm, sgxErrorArm, sgxErrorArm), true
	case "sgx-collateral-auth":
		kind, doc, chain := "TcbInfo", v.Collateral.TcbInfo, v.Collateral.TcbChain
		if v.Args[0] == "qe" {
			kind, doc, chain = "QeIdentity", v.Collateral.QeIdentity, v.Collateral.QeChain
		}
		return fmt.Sprintf(`match sgx.%s.parseVerified(s(%s), s(%s), s(%q), b(%s), %d) {
        Ok(_) -> "ok"
        %s
    }`, kind, blob(doc), blob(chain), v.Anchors, blob(v.Collateral.RootCrl), v.Now, sgxErrorArm), true
	case "sgx-crl":
		return fmt.Sprintf(`match x509.parseCrl(b(%q)) {
        Ok(list) -> match x509.parsePem(s(%q)) {
            Ok(issuers) -> match list.verifySignedBy(&issuers[0]) {
                Ok(_) -> match x509.parsePem(s(%q)) {
                    Ok(certs) -> "ok version=${list.version} revoked=${list.revoked.len()} number=${number(list.crlNumber)} unsupported=${list.unsupportedCritical.len()} revokes=${p(list.revokes(&certs[0]))}"
                    Err(_) -> "error:certificate"
                }
                Err(_) -> "error:verify"
            }
            Err(_) -> "error:issuer"
        }
        Err(_) -> "error:parse"
    }`, v.Hex, v.Anchors, v.Hex2), true
	case "sgx-pck-extension":
		return fmt.Sprintf(`match x509.parsePem(s(%q)) {
        Ok(certs) -> match sgx.PckExtension.of(&certs[0]) {
            Ok(e) -> "ok fmspc=${hex.encode(e.fmspc)} pceId=${hex.encode(e.pceId)} components=${hex.encode(e.components)} pceSvn=${e.pceSvn} cpuSvn=${hex.encode(e.cpuSvn)} type=${e.sgxType}"
            %s
        }
        Err(_) -> "error:certificate"
    }`, v.Hex, sgxErrorArm), true
	case "sgx-root":
		return fmt.Sprintf(`match x509.parsePem(s(%q)) {
        Ok(certs) -> root(&certs[0])
        Err(_) -> "error:parse"
    }`, v.Hex), true
	}
	return "", false
}

func emitVerify(v sgxVector, blob func(string) string) string {
	var body strings.Builder
	c := v.Collateral
	if c != nil {
		fmt.Fprintf(&body, `{
        let collateral = sgx.Collateral {
            tcbInfo: s(%s)
            tcbInfoIssuerChain: s(%s)
            qeIdentity: s(%s)
            qeIdentityIssuerChain: s(%s)
            pckCrl: b(%s)
            rootCaCrl: b(%s)
        }
`, blob(c.TcbInfo), blob(c.TcbChain), blob(c.QeIdentity), blob(c.QeChain), blob(c.PckCrl), blob(c.RootCrl))
	} else {
		body.WriteString(`{
        let collateral = sgx.Collateral {
            tcbInfo: ""
            tcbInfoIssuerChain: ""
            qeIdentity: ""
            qeIdentityIssuerChain: ""
            pckCrl: Bytes.zeros(0)
            rootCaCrl: Bytes.zeros(0)
        }
`)
	}
	fmt.Fprintf(&body, "        let mut policy = sgx.Policy.production(s(%q), %d, collateral, b(%q), sgx.Measurement.Enclave(b(%q)))\n", v.Anchors, v.Now, v.ReportData, v.Measurement)
	if c == nil {
		body.WriteString("        policy.collateral = None\n")
	}
	if v.Accept == "all" {
		body.WriteString("        policy.acceptedStatuses = sgx.AcceptableStatus.all()\n")
	}
	if v.Hex2 != "" {
		fmt.Fprintf(&body, "        policy.requireFmspc = Some(b(%q))\n", v.Hex2)
	}
	if v.MinEvaluation != 0 {
		fmt.Fprintf(&body, "        policy.minimumTcbEvaluationDataNumber = Some(%d)\n", v.MinEvaluation)
	}
	fmt.Fprintf(&body, `        match sgx.verify(b(%s), &policy) {
            Ok(r) -> verified(r)
            %s
        }
    }`, blob(v.Quote), sgxErrorArm)
	return body.String()
}
