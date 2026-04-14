package commands

type ExecResponse struct {
	OK        bool        `json:"ok"`
	Operation string      `json:"operation"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
}

type ExplainResponse struct {
	OK    bool   `json:"ok"`
	Query string `json:"query"`
	Plan  string `json:"plan"`
}

type ErrorResponse struct {
	OK        bool   `json:"ok"`
	Command   string `json:"command"`
	Query     string `json:"query,omitempty"`
	Error     string `json:"error"`
	ErrorType string `json:"error_type"`
}

type ConnectResponse struct {
	OK          bool   `json:"ok"`
	Command     string `json:"command"`
	URL         string `json:"url"`
	Connected   bool   `json:"connected"`
	Collections int    `json:"collections"`
	Message     string `json:"message"`
}

type DoctorResponse struct {
	OK          bool   `json:"ok"`
	Command     string `json:"command"`
	URL         string `json:"url"`
	Healthy     bool   `json:"healthy"`
	Collections int    `json:"collections"`
	Message     string `json:"message"`
}

type CommandResponse struct {
	OK      bool   `json:"ok"`
	Command string `json:"command"`
	Message string `json:"message"`
}

type VersionResponse struct {
	OK      bool   `json:"ok"`
	Command string `json:"command"`
	Version string `json:"version"`
	Message string `json:"message"`
}

type SearchHit struct {
	ID    string  `json:"id"`
	Score float32 `json:"score"`
	Text  string  `json:"text,omitempty"`
}
