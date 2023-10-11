package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/ArtisanCloud/PowerLibs/v3/http/helper"
	"github.com/ArtisanCloud/PowerWeChat/v3/src/kernel"
	"github.com/ArtisanCloud/PowerWeChat/v3/src/kernel/contract"
	"github.com/ArtisanCloud/PowerWeChat/v3/src/kernel/messages"
	"github.com/ArtisanCloud/PowerWeChat/v3/src/officialAccount"
	"github.com/ArtisanCloud/PowerWeChat/v3/src/work/server/handlers/models"
	"github.com/caarlos0/env/v9"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
	"github.com/sashabaranov/go-openai"
	"io"
	"net/http"
	"regexp"
	"strings"
)

type Config struct {
	MpAppid          string  `env:"MP_APPID" envDefault:""`                                 //公众号APPID
	MpSecret         string  `env:"MP_SECRET" envDefault:""`                                //公众号Secret
	MpToken          string  `env:"MP_TOKEN"  envDefault:""    `                            //公众号 token
	TelegramBotToken string  `env:"TELEGRAM_BOT_TOKEN"`                                     //TELEGRAM_BOT_TOKEN TODO
	DefaultApiUrl    string  `env:"DEFAULT_API_URL" envDefault:"https://api.aigc2d.com/v1"` //openai 接口地址
	DefaultKey       string  `env:"DEFAULT_KEY"`                                            //openai key
	DefaultWord      string  `env:"DEFAULT_WORD"`                                           //默认触发关键词
	DefaultModel     string  `env:"DEFAULT_MODEL" envDefault:"gpt-3.5-turbo"`               // 默认模型
	DefaultSystem    string  `env:"DEFAULT_MODEL" envDefault:""`                            //系统提示
	MaxToken         int     `env:"MAX_TOKEN"`                                              //最大 tokens
	Temperature      float32 `env:"TEMPERATURE" envDefault:"0.9"`                           //
	Stream           bool    `env:"STREAM" envDefault:"false"`                              //是否流输出
	ReplyChunkLength int     `env:"REPLY_CHUNK_LENGTH" envDefault:"1000"`                   //流输出每次输出限制
}

var commandHelp = []string{
	"命令：\n",
	"😻 /setKey=API_KEY - 设置OPENAI/AIGC2D",
	"😻 /setUrl=API_URL - 设置OPENAI/AIGC2D 接口地址,包含 /v1 ",
	"😻 /setWord=API_WORD - 设置问答触发词",
	"😻 /setSystem=SYSTEM_MESSAGE - 设置系统提示词",
	"😻 /setModel=MODEL_NAME - 设置模型名称",
}

var officialAccountApp *officialAccount.OfficialAccount
var config Config

func initWechat() {
	fmt.Println("初始化微信")
	account, err := officialAccount.NewOfficialAccount(&officialAccount.UserConfig{
		AppID:  config.MpAppid,
		Secret: config.MpSecret,
		Token:  config.MpToken,
		//Debug:  false,
		//HttpDebug: true,
	})
	if err != nil {
		return
	}
	officialAccountApp = account
}

func initConfig() {
	err := env.Parse(&config)
	if err != nil {
		return
	}
	fmt.Println(config)
}

func Wechat(c *gin.Context) {
	_, ok := c.GetQuery("echostr")
	if ok {
		//验证url
		response, err := officialAccountApp.Server.VerifyURL(c.Request)
		if err != nil {
			return
		}
		text, err := io.ReadAll(response.Body)
		if err != nil {
			return
		}
		c.String(http.StatusOK, string(text))
		return
	}

	notify, err := officialAccountApp.Server.Notify(c.Request, func(event contract.EventInterface) interface{} {
		if event.GetMsgType() == "text" {
			messageText := models.MessageText{}
			err := event.ReadMessage(&messageText)
			if err != nil {
				return nil
			}
			content := messageText.Content
			openid := event.GetFromUserName()
			re := regexp.MustCompile("^/(\\w+)=(.*?)($|\\s)")
			match := re.FindStringSubmatch(content)
			if len(match) > 1 {
				fmt.Println(match)
				key := match[1]
				value := match[2]
				switch strings.ToLower(key) {
				case "setkey":
					//设置 apikey
					config.DefaultKey = value
					return messages.NewText("key 设置成功")
				case "seturl":
					config.DefaultApiUrl = value
					return messages.NewText("url 设置成功")
				case "setword":
					config.DefaultWord = value
					return messages.NewText("word 设置成功")
				case "setsystem":
					config.DefaultSystem = value
					return messages.NewText("system 设置成功")
				case "setmodel":
					config.DefaultSystem = value
					return messages.NewText("model 设置成功")
				default:
					return messages.NewText("错误的指令")
				}
			}

			if content == "/help" {
				return messages.NewText(strings.Join(commandHelp, "\n"))
			}

			go llmReply(c, content, openid, "wechat")
		}
		fmt.Println(event.GetEvent())

		return kernel.SUCCESS_EMPTY_RESPONSE
	})
	if err != nil {
		panic(err)
	}

	err = helper.HttpResponseSend(notify, c.Writer)
	if err != nil {
		panic(err)
	}
	return
}

func sendTyping(ctx context.Context, openid string) {
	_, err := officialAccountApp.CustomerService.ShowTypingStatusToUser(ctx, openid)
	if err != nil {
		_ = fmt.Sprintf("sendTyping error:%s", err.Error())
		return
	}
}
func llmReply(c context.Context, input, openid, botType string) {
	if botType == "wechat" {
		sendTyping(c, openid)
	}
	var completionMessages []openai.ChatCompletionMessage
	if len(config.DefaultSystem) > 0 {
		completionMessages = append(completionMessages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: config.DefaultSystem,
		})
	}
	completionMessages = append(completionMessages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: input,
	})

	clientConfig := openai.DefaultConfig(config.DefaultKey)
	clientConfig.BaseURL = config.DefaultApiUrl
	openaiClient := openai.NewClientWithConfig(clientConfig)

	request := openai.ChatCompletionRequest{
		Model:    config.DefaultModel,
		Messages: completionMessages,
	}
	if config.MaxToken > 0 {
		request.MaxTokens = config.MaxToken
	}
	if config.Temperature > 0 {
		request.Temperature = config.Temperature
	}
	if config.Stream {
		stream, err := openaiClient.CreateChatCompletionStream(c, request)
		if err != nil {
			sendMessage(c, openid, fmt.Sprintf("api err:%s", err.Error()))
			return
		}
		defer stream.Close()
		retContent := ""
		buffer := ""
		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				buffer += "◾️"
				sendMessage(c, openid, buffer)
				return
			}
			if err != nil {
				sendMessage(c, openid, fmt.Sprintf("api err:%s", err.Error()))
				return
			}
			fmt.Println(response)
			if len(response.Choices) > 0 {
				buffer += response.Choices[0].Delta.Content
			}
			if len(buffer) > config.ReplyChunkLength {
				retContent = buffer + "..."
				buffer = ""
				sendMessage(c, openid, retContent)
			}
		}

	} else {
		completion, err := openaiClient.CreateChatCompletion(c, request)
		if err != nil {
			sendMessage(c, openid, fmt.Sprintf("api err:%s", err.Error()))
			return
		}
		chunk := splitString(completion.Choices[0].Message.Content, config.ReplyChunkLength)
		for i := 0; i < len(chunk); i++ {
			sendMessage(c, openid, chunk[0])
		}

	}

	openai.NewClientWithConfig(clientConfig)
}

func sendMessage(ctx context.Context, openid, content string) {
	officialAccountApp.CustomerService.Message(ctx, messages.NewText(content)).SetTo(openid).Send(ctx)
}
func splitString(str string, length int) []string {
	var result []string
	for i := 0; i < len(str); i += length {
		end := i + length
		if end > len(str) {
			end = len(str)
		}
		result = append(result, str[i:end])
	}
	return result
}

func main() {
	initConfig()
	initWechat()
	r := gin.Default()
	r.Any("/wechat", Wechat)
	r.Run(":8080")
}
