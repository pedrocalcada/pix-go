package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"pix-go/configuration"
	"pix-go/types"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

var saldoTotal float64 = 100 // Variável global para manter a soma do saldo

var client openai.Client

func main() {

	config, err := configuration.InitConfig()
	if err != nil {
		log.Fatalf("Erro ao carregar configuração: %v", err)
	}

	client = openai.NewClient(
		option.WithAPIKey(config.GetString("secret")),
	)

	// Buffer de contexto por sessão (simples, por IP para exemplo)
	// contextBuffers := make(map[string][]ChatMessage)
	buffer := make([]types.ChatMessage, 0)

	http.HandleFunc("/getBuffer", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(buffer); err != nil {
			http.Error(w, "Erro ao serializar buffer: "+err.Error(), http.StatusInternalServerError)
		}
	})

	http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		var req types.ChatRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Adiciona mensagem do usuário ao buffer
		// buffer = append(buffer, types.ChatMessage{
		// 	Role:    "user",
		// 	Content: req.Message,
		// })
		content, err := chamadaOpenAIComContexto(req.Message, config.GetString("classificadorPrompt"), openai.ChatModelGPT4_1Nano, &buffer)
		if err != nil {
			http.Error(w, "OpenAI error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(content)
		log.Printf("Intenção classificada: %s", content)

		// switch content {
		// case "saldo":
		// 	log.Println("Intenção: Saldo")
		// 	buffer = append(buffer, types.ChatMessage{
		// 		Role:    "assistant",
		// 		Content: "saldo: " + strconv.FormatFloat(saldoTotal, 'f', 2, 64),
		// 	})
		// 	json.NewEncoder(w).Encode("saldo: " + strconv.FormatFloat(saldoTotal, 'f', 2, 64))
		// case "pix":
		// 	log.Println("Intenção: Pix")

		// 	content, err := chamadaOpenAIComContexto(req.Message, config.GetString("pixPrompt"), openai.ChatModelGPT4_1Mini, &buffer)
		// 	var respPix types.InfoPIX
		// 	if err != nil {
		// 		http.Error(w, "Erro ao chamar OpenAI: "+err.Error(), http.StatusInternalServerError)
		// 		return
		// 	}
		// 	if err := json.Unmarshal([]byte(content), &respPix); err != nil {
		// 		http.Error(w, "Resposta da OpenAI não é um JSON válido: "+err.Error(), http.StatusInternalServerError)
		// 		log.Printf("Erro ao decodificar JSON: %v", content)
		// 		return
		// 	}
		// 	log.Printf("Chave Pix: %s, Valor: %.2f", respPix.Chave, respPix.Valor)

		// 	json.NewEncoder(w).Encode("Você está prestes a fazer um Pix para " + respPix.Chave + " no valor de R$ " + strconv.FormatFloat(respPix.Valor, 'f', 2, 64) + ". Confirma?")
		// case "limite":
		// 	log.Println("Intenção: Limite")
		// 	json.NewEncoder(w).Encode("alteração de limite")
		// default:
		// 	log.Println("Intenção desconhecida")
		// }
		w.Header().Set("Content-Type", "application/json")

		log.Printf("Chat completion took %s", time.Since(start))
	})

	log.Println("Server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// Nova função para chamada OpenAI com contexto
func chamadaOpenAIComContexto(message string, prompt string, model openai.ChatModel, buffer *[]types.ChatMessage) (string, error) {
	var messages []openai.ChatCompletionMessageParamUnion

	// *buffer = append(*buffer, types.ChatMessage{
	// 	Role:    "system",
	// 	Content: prompt,
	// })
	messages = append(messages, openai.SystemMessage(prompt))
	messages = append(messages, openai.UserMessage(message))
	// Adiciona histórico do buffer
	// for _, msg := range *buffer {
	// 	switch msg.Role {
	// 	case "system":
	// 		messages = append(messages, openai.SystemMessage(msg.Content))
	// 		continue
	// 	case "user":
	// 		messages = append(messages, openai.UserMessage(msg.Content))
	// 	case "assistant":
	// 		messages = append(messages, openai.AssistantMessage(msg.Content))
	// 	}
	// }
	chatCompletion, err := client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    model,
	})
	if err != nil {
		return "", err
	}

	content := chatCompletion.Choices[0].Message.Content
	// Adiciona resposta da OpenAI ao buffer
	// *buffer = append(*buffer, types.ChatMessage{
	// 	Role:    "assistant",
	// 	Content: content,
	// })
	return content, nil
}
