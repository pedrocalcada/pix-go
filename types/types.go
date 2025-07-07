package types

type ChatRequest struct {
	Message string `json:"message"`
}

type InfoPIX struct {
	Chave string  `json:"chave_pix"`
	Valor float64 `json:"valor"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OllamaRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type OllamaResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}
