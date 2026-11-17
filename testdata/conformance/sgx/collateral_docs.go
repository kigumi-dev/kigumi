package main

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

const pcsTime = "2006-01-02T15:04:05Z"

type componentJSON struct {
	Svn      int    `json:"svn"`
	Category string `json:"category,omitempty"`
}

type sgxTcbJSON struct {
	Components []componentJSON `json:"sgxtcbcomponents"`
	PceSvn     int             `json:"pcesvn"`
}

type tcbLevelJSON struct {
	Tcb        sgxTcbJSON `json:"tcb"`
	TcbDate    string     `json:"tcbDate"`
	TcbStatus  string     `json:"tcbStatus"`
	AdvisoryID []string   `json:"advisoryIDs,omitempty"`
}

type tcbInfoJSON struct {
	ID                      string         `json:"id"`
	Version                 int            `json:"version"`
	IssueDate               string         `json:"issueDate"`
	NextUpdate              string         `json:"nextUpdate"`
	Fmspc                   string         `json:"fmspc"`
	PceID                   string         `json:"pceId"`
	TcbType                 int            `json:"tcbType"`
	TcbEvaluationDataNumber int            `json:"tcbEvaluationDataNumber"`
	TcbLevels               []tcbLevelJSON `json:"tcbLevels"`
}

type qeTcbJSON struct {
	IsvSvn int `json:"isvsvn"`
}

type qeLevelJSON struct {
	Tcb        qeTcbJSON `json:"tcb"`
	TcbDate    string    `json:"tcbDate"`
	TcbStatus  string    `json:"tcbStatus"`
	AdvisoryID []string  `json:"advisoryIDs,omitempty"`
}

type qeIdentityJSON struct {
	ID                      string        `json:"id"`
	Version                 int           `json:"version"`
	IssueDate               string        `json:"issueDate"`
	NextUpdate              string        `json:"nextUpdate"`
	TcbEvaluationDataNumber int           `json:"tcbEvaluationDataNumber"`
	Miscselect              string        `json:"miscselect"`
	MiscselectMask          string        `json:"miscselectMask"`
	Attributes              string        `json:"attributes"`
	AttributesMask          string        `json:"attributesMask"`
	Mrsigner                string        `json:"mrsigner"`
	IsvProdID               int           `json:"isvprodid"`
	TcbLevels               []qeLevelJSON `json:"tcbLevels"`
}

type level struct {
	components [16]byte
	pceSvn     int
	status     string
	date       time.Time
	advisories []string
}

func tcbInfoBody(fmspc, pceID []byte, issued, next time.Time, evaluation int, levels []level) tcbInfoJSON {
	doc := tcbInfoJSON{
		ID: "SGX", Version: 3, IssueDate: issued.Format(pcsTime), NextUpdate: next.Format(pcsTime),
		Fmspc: strings.ToUpper(hex.EncodeToString(fmspc)), PceID: strings.ToUpper(hex.EncodeToString(pceID)),
		TcbEvaluationDataNumber: evaluation,
	}
	for _, l := range levels {
		var components []componentJSON
		for i, svn := range l.components {
			c := componentJSON{Svn: int(svn)}
			if i == 0 {
				c.Category = "BIOS"
			}
			components = append(components, c)
		}
		doc.TcbLevels = append(doc.TcbLevels, tcbLevelJSON{
			Tcb: sgxTcbJSON{Components: components, PceSvn: l.pceSvn}, TcbDate: l.date.Format(pcsTime),
			TcbStatus: l.status, AdvisoryID: l.advisories,
		})
	}
	return doc
}

// signedDocument signs the compact body as PCS publishes it; `edit` changes the body after signing.
func signedDocument(key string, body any, by *ecdsa.PrivateKey, edit func([]byte) []byte) []byte {
	raw := must(json.Marshal(body))
	signature := hex.EncodeToString(sign(by, raw))
	if edit != nil {
		raw = edit(raw)
	}
	return []byte(`{"` + key + `":` + string(raw) + `,"signature":"` + signature + `"}`)
}
