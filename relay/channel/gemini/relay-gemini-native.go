package gemini

import (
	"fmt"
	"io"
	"net/http"
	"one-api/common"
	"one-api/dto"
	relaycommon "one-api/relay/common"
	"one-api/relay/helper"
	"one-api/service"
	"strings"

	"github.com/gin-gonic/gin"
)

func GeminiTextGenerationHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *dto.OpenAIErrorWithStatusCode) {
	defer service.CloseResponseBodyGracefully(resp)
	
	// 读取响应体
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, service.OpenAIErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, service.OpenAIErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError)
	}

	if common.DebugEnabled {
		println(string(responseBody))
	}

	// 解析为 Gemini 原生响应格式
	var geminiResponse GeminiChatResponse
	err = common.DecodeJson(responseBody, &geminiResponse)
	if err != nil {
		return nil, service.OpenAIErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
	}

	// 计算使用量（基于 UsageMetadata）
	usage := dto.Usage{
		PromptTokens:     geminiResponse.UsageMetadata.PromptTokenCount + geminiResponse.UsageMetadata.ToolUsePromptTokenCount,
		CompletionTokens: geminiResponse.UsageMetadata.CandidatesTokenCount + geminiResponse.UsageMetadata.ThoughtsTokenCount,
		TotalTokens:      geminiResponse.UsageMetadata.TotalTokenCount,
	}

	usage.CompletionTokenDetails.ReasoningTokens = geminiResponse.UsageMetadata.ThoughtsTokenCount

	if usage.PromptTokens == 0 && usage.TotalTokens != 0 {
		usage.PromptTokens = usage.TotalTokens - usage.CompletionTokens
	}

	for _, detail := range geminiResponse.UsageMetadata.PromptTokensDetails {
		if detail.Modality == "AUDIO" {
			usage.PromptTokensDetails.AudioTokens = detail.TokenCount
		} else if detail.Modality == "TEXT" {
			usage.PromptTokensDetails.TextTokens = detail.TokenCount
		}
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)
	
	return &usage, nil
}

func GeminiTextGenerationStreamHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *dto.OpenAIErrorWithStatusCode) {
	var usage = &dto.Usage{}
	var imageCount int

	helper.SetEventStreamHeaders(c)

	responseText := strings.Builder{}

	helper.StreamScannerHandler(c, resp, info, func(data string) bool {
		var geminiResponse GeminiChatResponse
		err := common.DecodeJsonStr(data, &geminiResponse)
		if err != nil {
			common.LogError(c, "error unmarshalling stream response: "+err.Error())
			return false
		}

		// 统计图片数量
		for _, candidate := range geminiResponse.Candidates {
			for _, part := range candidate.Content.Parts {
				if part.InlineData != nil && part.InlineData.MimeType != "" {
					imageCount++
				}
				if part.Text != "" {
					responseText.WriteString(part.Text)
				}
			}
		}

		// 更新使用量统计
		if geminiResponse.UsageMetadata.TotalTokenCount != 0 {
			usage.PromptTokens = geminiResponse.UsageMetadata.PromptTokenCount + geminiResponse.UsageMetadata.ToolUsePromptTokenCount
			usage.CompletionTokens = geminiResponse.UsageMetadata.CandidatesTokenCount + geminiResponse.UsageMetadata.ThoughtsTokenCount
			usage.TotalTokens = geminiResponse.UsageMetadata.TotalTokenCount
			usage.CompletionTokenDetails.ReasoningTokens = geminiResponse.UsageMetadata.ThoughtsTokenCount
			
			if usage.PromptTokens == 0 && usage.TotalTokens != 0 {
				usage.PromptTokens = usage.TotalTokens - usage.CompletionTokens
			}

			for _, detail := range geminiResponse.UsageMetadata.PromptTokensDetails {
				if detail.Modality == "AUDIO" {
					usage.PromptTokensDetails.AudioTokens = detail.TokenCount
				} else if detail.Modality == "TEXT" {
					usage.PromptTokensDetails.TextTokens = detail.TokenCount
				}
			}
		}

		// 直接发送 GeminiChatResponse 响应
		err = helper.StringData(c, data)
		if err != nil {
			common.LogError(c, err.Error())
		}

		return true
	})

	if imageCount != 0 {
		if usage.CompletionTokens == 0 {
			usage.CompletionTokens = imageCount * 258
		}
	}

	// 保底计算：如果 PromptTokens 为 0，尝试从原始请求重新计算
	// 确保计费准确性
	if usage.PromptTokens == 0 {
		if reqInterface, exists := c.Get("gemini_request"); exists {
			if geminiReq, ok := reqInterface.(*GeminiChatRequest); ok {
				// 计算真实的输入 tokens
				var inputTexts []string
				for _, content := range geminiReq.Contents {
					for _, part := range content.Parts {
						if part.Text != "" {
							inputTexts = append(inputTexts, part.Text)
						}
					}
				}
				if len(inputTexts) > 0 {
					inputText := strings.Join(inputTexts, "\n")
					usage.PromptTokens = service.CountTokenInput(inputText, info.UpstreamModelName)
					if common.DebugEnabled {
						common.LogInfo(c, fmt.Sprintf("fallback calculated PromptTokens: %d", usage.PromptTokens))
					}
				}
			}
		}
	}

	
	// 如果usage.CompletionTokens为0，则使用本地统计的completion tokens
	if usage.CompletionTokens == 0 {
		usage = service.ResponseText2Usage(responseText.String(), info.UpstreamModelName, usage.PromptTokens)
	}
	// 移除流式响应结尾的[Done]，因为Gemini API没有发送Done的行为
	//helper.Done(c)

	return usage, nil
}
