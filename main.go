package main

import (
	"ai_bot/plugins"
	"context"
	"encoding/json"
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
	"github.com/syndtr/goleveldb/leveldb"
	"io"
	"net/http"
	"regexp"
	"strconv"
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
	DefaultSystem    string  `env:"DEFAULT_SYSTEM" envDefault:""`                           //系统提示
	MaxToken         int     `env:"MAX_TOKEN"`                                              //最大 tokens
	Temperature      float32 `env:"TEMPERATURE" envDefault:"0.9"`                           //
	Stream           bool    `env:"STREAM" envDefault:"false"`                              //是否流输出
	ReplyChunkLength int     `env:"REPLY_CHUNK_LENGTH" envDefault:"1000"`                   //流输出每次输出限制
	EnableHistory    bool    `env:"ENABLE_HISTORY"`
	EnableSearch     bool    `env:"ENABLE_SEARCH"`
	SerperKey        string  `env:"SERPER_KEY"` //serper.dev apikey
}

var commandHelp = []string{
	"命令：\n",
	"😻 /setKey=API_KEY - 设置OPENAI/AIGC2D",
	"😻 /setUrl=API_URL - 设置OPENAI/AIGC2D 接口地址,包含 /v1 ",
	"😻 /setWord=API_WORD - 设置问答触发词",
	"😻 /setSystem=SYSTEM_MESSAGE - 设置系统提示词",
	"😻 /setModel=MODEL_NAME - 设置模型名称",
	"😻 /setHistory=true - 启用历史消息",
	"😻 /setSearchKey=SERPER_KEY - 设置搜索key",
	"😻 /setSearch=true - 启用搜索",
	"😻 /clear - 清除历史对话消息",
}

var officialAccountApp *officialAccount.OfficialAccount
var db *leveldb.DB
var config Config

func initDB() {
	file, err := leveldb.OpenFile("./data/level/db", nil)
	if err != nil {
		fmt.Println("db 初始化失败")
		return
	}
	db = file
}

func initConfig() {
	err := env.Parse(&config)
	if err != nil {
		return
	}
	fmt.Println(config)
}

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
					config.DefaultModel = value
					return messages.NewText("model 设置成功")
				case "sethistory":
					config.EnableHistory, _ = strconv.ParseBool(value)
					return messages.NewText("history 设置成功")
				case "setsearch":
					config.EnableSearch, _ = strconv.ParseBool(value)
					return messages.NewText("search 设置成功")
				case "setsearchkey":
					config.SerperKey = value
					return messages.NewText("search key 设置成功")
				default:
					return messages.NewText("错误的指令")
				}
			}

			if content == "/help" {
				return messages.NewText(strings.Join(commandHelp, "\n"))
			}
			if content == "/clear" {
				if config.EnableHistory {
					_ = db.Delete(getDBKey(openid), nil)
					return messages.NewText("对话已清除")
				}
				return messages.NewText("未启用历史对话消息")

			}

			if len(config.DefaultKey) == 0 || len(config.DefaultApiUrl) == 0 {
				apikey := "✅"
				apiurl := "✅"
				search := "✅"
				if len(config.DefaultKey) == 0 {
					apikey = "❌"
				}
				if len(config.DefaultApiUrl) == 0 {
					apiurl = "❌"
				}
				if len(config.SerperKey) == 0 {
					search = "❌"
				}

				return messages.NewText(fmt.Sprintf("请先设置APIKEY:[%s] - API_URL:[%s]  - Search:[%s]  - API_WORD:%s \n\n%s", apikey, apiurl, search, config.DefaultWord, strings.Join(commandHelp, "\n")))
			}

			if len(config.DefaultWord) > 0 {
				if strings.Contains(content, config.DefaultWord) {
					content = strings.Replace(content, config.DefaultWord, "", 1)
				} else {
					return kernel.SUCCESS_EMPTY_RESPONSE
				}
			}

			//
			var completionMessages []openai.ChatCompletionMessage
			if config.EnableHistory {
				//启用历史消息
				history := readHistory(openid)
				completionMessages = history
			}
			if len(config.DefaultSystem) > 0 && len(completionMessages) == 0 {
				completionMessages = append(completionMessages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleSystem,
					Content: config.DefaultSystem,
				})
			}
			completionMessages = append(completionMessages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: content,
			})

			go llmReply(c, completionMessages, openid, "wechat")
		}

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

func llmReply(c context.Context, messages []openai.ChatCompletionMessage, openid, botType string) {
	if botType == "wechat" {
		sendTyping(c, openid)
	}
	clientConfig := openai.DefaultConfig(config.DefaultKey)
	clientConfig.BaseURL = config.DefaultApiUrl
	openaiClient := openai.NewClientWithConfig(clientConfig)

	request := openai.ChatCompletionRequest{
		Model:    config.DefaultModel,
		Messages: messages,
	}
	var functions []openai.FunctionDefinition
	if config.EnableSearch && len(config.SerperKey) != 0 {
		//启用搜索
		//request.Functions[0] =
		functions = append(functions, plugins.SearchEngineFunction())
	}
	request.Functions = functions

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
		completeContent := ""
		functionName := ""
		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				if len(functionName) != 0 {
					sendMessage(c, openid, fmt.Sprintf("已触发[%s]插件", functionName))
					if functionName == "search-engine" {
						var arg plugins.SearchEngineArguments
						_ = json.Unmarshal([]byte(completeContent), &arg)
						searchResult, err := plugins.SearchEngine(arg.Query, config.SerperKey)
						if err != nil {
							sendMessage(c, openid, "search-engine err:"+err.Error())
							return
						}
						messages = append(messages, openai.ChatCompletionMessage{
							Role:    openai.ChatMessageRoleFunction,
							Content: searchResult,
							Name:    "search-engine",
						})
						llmReply(c, messages, openid, botType)
					}
				} else {
					buffer += "◾️"
					sendMessage(c, openid, buffer)
				}
				break
			}
			if err != nil {
				sendMessage(c, openid, fmt.Sprintf("api err:%s", err.Error()))
				break
			}

			if len(response.Choices) > 0 {
				if response.Choices[0].Delta.FunctionCall != nil {
					if len(response.Choices[0].Delta.FunctionCall.Name) > 0 {
						functionName = response.Choices[0].Delta.FunctionCall.Name
					}
					completeContent += response.Choices[0].Delta.FunctionCall.Arguments
				} else {
					buffer += response.Choices[0].Delta.Content
					completeContent += response.Choices[0].Delta.Content
					if len(buffer) > config.ReplyChunkLength {
						retContent = buffer + "..."
						buffer = ""
						sendMessage(c, openid, retContent)
					}
				}
			}

		}
		if config.EnableHistory && functionName == "" {
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: completeContent,
			})
			writeHistory(openid, messages)
		}

	} else {
		completion, err := openaiClient.CreateChatCompletion(c, request)
		if err != nil {
			sendMessage(c, openid, fmt.Sprintf("api err:%s", err.Error()))
			return
		}
		if completion.Choices[0].FinishReason == "function_call" {
			//调用函数
			name := completion.Choices[0].Message.FunctionCall.Name
			args := completion.Choices[0].Message.FunctionCall.Arguments
			sendMessage(c, openid, fmt.Sprintf("已触发[%s]插件", name))
			if name == "search-engine" {
				var arg plugins.SearchEngineArguments
				_ = json.Unmarshal([]byte(args), &arg)
				searchResult, err := plugins.SearchEngine(arg.Query, config.SerperKey)
				if err != nil {
					sendMessage(c, openid, "search-engine err:"+err.Error())
					return
				}
				messages = append(messages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleFunction,
					Content: searchResult,
					Name:    "search-engine",
				})
				llmReply(c, messages, openid, botType)
			}
		} else {
			chunk := splitString(completion.Choices[0].Message.Content, config.ReplyChunkLength)
			for i := 0; i < len(chunk); i++ {
				sendMessage(c, openid, chunk[0])
			}

			if config.EnableHistory {
				messages = append(messages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleAssistant,
					Content: completion.Choices[0].Message.Content,
				})
				writeHistory(openid, messages)
			}
		}
	}
}

func getDBKey(openid string) []byte {
	return []byte(fmt.Sprintf("HISTORY_MESSAGE_" + openid))
}

func writeHistory(openid string, messages []openai.ChatCompletionMessage) {
	marshal, err := json.Marshal(messages)
	if err != nil {
		return
	}
	err = db.Put(getDBKey(openid), marshal, nil)
	if err != nil {
		return
	}
}

func readHistory(openid string) (history []openai.ChatCompletionMessage) {
	data, err := db.Get(getDBKey(openid), nil)
	if err != nil {
		return nil
	}
	err = json.Unmarshal(data, &history)
	if err != nil {
		return nil
	}
	return
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
	initDB()
	initConfig()
	initWechat()
	r := gin.Default()
	r.Any("/wechat", Wechat)
	r.Run(":8080")
}
