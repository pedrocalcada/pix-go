package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"pix-go/configuration"
	"pix-go/types"
	"strings"
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

	http.HandleFunc("/randomMessage", func(w http.ResponseWriter, r *http.Request) {
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
		w.Header().Set("Content-Type", "application/json")
		content, err := chamadaOpenAIComContexto(req.Message, config.GetString("randomMessage"), "gpt-4.1-2025-04-14", &buffer)
		if err != nil {
			http.Error(w, "OpenAI error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(content)
		log.Printf("Mensagem Random: %s", content)
	})

	http.HandleFunc("/randomMessageOllama", func(w http.ResponseWriter, r *http.Request) {
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
		w.Header().Set("Content-Type", "application/json")

		// Prompt para o Ollama
		ollamaPrompt := `Você é um agente bancário especializado em operações de PIX.
Suas funções são:
- Identificar chave e valor de um PIX,
- Identificar se o cliente deseja consultar o saldo,
- Identificar se o cliente deseja alterar o limite transacional do PIX.

Sempre responda em JSON com os seguintes campos:
- "intencao": pode ser "pix", "saldo" ou "limite"
- Se "intencao" for "pix", inclua também "valor" (número) e "chave_pix" (string)
- Se "intencao" for "limite", inclua também "novo_limite" (número)
- Se "intencao" for "saldo", apenas o campo "intencao" é necessário

Exemplo de resposta para um PIX:
{"intencao":"pix","valor":100.50,"chave_pix":"email@exemplo.com"}

Exemplo para consulta de saldo:
{"intencao":"saldo"}

Exemplo para alteração de limite:
{"intencao":"limite","novo_limite":2000.00}

Responda apenas o JSON, sem explicações.`

		// Monta o payload para o Ollama
		ollamaPayload := map[string]interface{}{
			"model":  "qwen3:8b",
			"think":  false,
			"stream": false,
			"messages": []map[string]string{
				{"role": "system", "content": ollamaPrompt},
				{"role": "user", "content": req.Message},
			},
		}
		payloadBytes, err := json.Marshal(ollamaPayload)
		if err != nil {
			http.Error(w, "Failed to marshal payload", http.StatusInternalServerError)
			return
		}

		// Faz a requisição HTTP para o Ollama
		resp, err := http.Post("http://localhost:11434/v1/chat", "application/json",
			bytes.NewReader(payloadBytes))
		if err != nil {
			http.Error(w, "Erro ao chamar Ollama: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "Erro ao ler resposta do Ollama: "+err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("Resposta do Ollama: %s", respBody)

		// Extrai o conteúdo da resposta do Ollama
		var ollamaResp struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
			http.Error(w, "Resposta do Ollama não é válida: "+err.Error(), http.StatusInternalServerError)
			return
		}

		content := ollamaResp.Choices[0].Message.Content

		// Remove prefixo "json" se existir
		content = strings.TrimSpace(content)
		if strings.HasPrefix(content, "json") {
			content = strings.TrimSpace(content[4:])
		}

		// Valida e envia o JSON
		var jsonResp map[string]interface{}
		if err := json.Unmarshal([]byte(content), &jsonResp); err != nil {
			http.Error(w, "Resposta do Ollama não é um JSON válido: "+err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(jsonResp)
		log.Printf("Mensagem Random Ollama: %s", content)
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
		content, err := chamadaOpenAISemContexto(req.Message, config.GetString("classificadorPrompt"), "gpt-4.1-2025-04-14")
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
	for _, msg := range *buffer {
		switch msg.Role {
		case "system":
			messages = append(messages, openai.SystemMessage(msg.Content))
			continue
		case "user":
			messages = append(messages, openai.UserMessage(msg.Content))
		case "assistant":
			messages = append(messages, openai.AssistantMessage(msg.Content))
		}
	}
	chatCompletion, err := client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    model,
	})
	if err != nil {
		return "", err
	}

	content := chatCompletion.Choices[0].Message.Content
	// Adiciona resposta da OpenAI ao buffer
	*buffer = append(*buffer, types.ChatMessage{
		Role:    "assistant",
		Content: content,
	})
	return content, nil
}

// Nova função para chamada OpenAI com contexto
func chamadaOpenAISemContexto(message string, prompt string, model openai.ChatModel) (string, error) {
	var messages []openai.ChatCompletionMessageParamUnion

	// *buffer = append(*buffer, types.ChatMessage{
	// 	Role:    "system",
	// 	Content: prompt,
	// })
	messages = append(messages, openai.SystemMessage(prompt))
	messages = append(messages, openai.UserMessage(message))

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
