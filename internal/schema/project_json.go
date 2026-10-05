package schema

import "encoding/json"

// ProjectJSON renders mod; byte-identical input renders byte-identical
// output.
func ProjectJSON(mod *Module) ([]byte, error) {
	return json.MarshalIndent(mod, "", "  ")
}

// DecodeModule fails closed via DecodeEnvelope before trusting the rest.
func DecodeModule(data []byte) (*Module, error) {
	if _, err := DecodeEnvelope(data); err != nil {
		return nil, err
	}
	var mod Module
	if err := json.Unmarshal(data, &mod); err != nil {
		return nil, err
	}
	return &mod, nil
}
