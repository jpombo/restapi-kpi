package ollama

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Client struct {
	BaseURL    string
	ModelName  string
	HTTPClient *http.Client
}

type GenerateRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	Stream      bool    `json:"stream"`
	Temperature float64 `json:"temperature"`
}

type GenerateResponse struct {
	Response string `json:"response"`
}

func NewClient(baseURL, modelName string) *Client {
	return &Client{
		BaseURL:   baseURL,
		ModelName: modelName,
		// AUMENTAR TIMEOUT PARA 5 MINUTOS (300 segundos)
		HTTPClient: &http.Client{
			Timeout: 300 * time.Second, // 5 minutos
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// Versão com timeout configurável
func NewClientWithTimeout(baseURL, modelName string, timeoutSeconds int) *Client {
	return &Client{
		BaseURL:   baseURL,
		ModelName: modelName,
		HTTPClient: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

func (c *Client) GenerateRequirements(kpiJSON string) (string, error) {
	url := fmt.Sprintf("%s/api/generate", c.BaseURL)

	reqBody := GenerateRequest{
		Model:       c.ModelName,
		Prompt:      kpiJSON,
		Stream:      false,
		Temperature: 0.3,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("erro ao serializar request: %w", err)
	}

	fmt.Printf("Enviando requisição para Ollama (timeout: %v)...\n", c.HTTPClient.Timeout)

	resp, err := c.HTTPClient.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("erro ao chamar Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama retornou status %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("erro ao decodificar resposta: %w", err)
	}

	return ollamaResp.Response, nil
}

// Versão com streaming (recomendado para respostas longas)
func (c *Client) GenerateRequirementsStream(kpiJSON string) (string, error) {
	url := fmt.Sprintf("%s/api/generate", c.BaseURL)

	reqBody := GenerateRequest{
		Model:       c.ModelName,
		Prompt:      kpiJSON,
		Stream:      true, // Habilita streaming
		Temperature: 0.3,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("erro ao serializar request: %w", err)
	}

	resp, err := c.HTTPClient.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("erro ao chamar Ollama: %w", err)
	}
	defer resp.Body.Close()

	var fullResponse string
	decoder := json.NewDecoder(resp.Body)

	for decoder.More() {
		var streamResp struct {
			Response string `json:"response"`
			Done     bool   `json:"done"`
		}

		if err := decoder.Decode(&streamResp); err != nil {
			return "", fmt.Errorf("erro ao decodificar stream: %w", err)
		}

		fullResponse += streamResp.Response

		if streamResp.Done {
			break
		}
	}

	return fullResponse, nil
}

// ollama/client.go - Adicionar logs detalhados
func (c *Client) GenerateRequirementsWithProgress(kpiJSON string) (string, error) {
	url := fmt.Sprintf("%s/api/generate", c.BaseURL)

	reqBody := GenerateRequest{
		Model:       c.ModelName,
		Prompt:      kpiJSON,
		Stream:      true, // Usar streaming para ver progresso
		Temperature: 0.3,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	startTime := time.Now()
	log.Printf("[Ollama] Iniciando requisição para modelo: %s", c.ModelName)

	resp, err := c.HTTPClient.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Printf("[Ollama] Erro após %v: %v", time.Since(startTime), err)
		return "", err
	}
	defer resp.Body.Close()

	log.Printf("[Ollama] Conectado, processando resposta...")

	var fullResponse string
	var chunkCount int
	decoder := json.NewDecoder(resp.Body)

	for decoder.More() {
		var streamResp struct {
			Response string `json:"response"`
			Done     bool   `json:"done"`
		}

		if err := decoder.Decode(&streamResp); err != nil {
			return "", fmt.Errorf("erro ao decodificar: %w", err)
		}

		fullResponse += streamResp.Response
		chunkCount++

		if chunkCount%10 == 0 {
			log.Printf("[Ollama] Recebidos %d chunks até agora...", chunkCount)
		}

		if streamResp.Done {
			break
		}
	}

	log.Printf("[Ollama] Concluído em %v, recebidos %d bytes", time.Since(startTime), len(fullResponse))

	return fullResponse, nil
}
