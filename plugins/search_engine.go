package plugins

import (
	"encoding/json"
	"errors"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
	"io/ioutil"
	"net/http"
	"strings"
)

// SearchEngineResponse 只保留需要的数据
type SearchEngineResponse struct {
	KnowledgeGraph any `json:"knowledgeGraph,omitempty"`
	Organic        any `json:"organic,omitempty"`
	AnswerBox      any `json:"answerBox,omitempty"`
}
type SearchEngineArguments struct {
	Query string `json:"query"`
}

func SearchEngine(ql string, key string) (string, error) {
	url := "https://google.serper.dev/search"
	method := "POST"

	payload := strings.NewReader(`{"q":"` + ql + `","gl":"cn","hl":"zh-cn"}`)

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		return "", err
	}
	req.Header.Add("X-API-KEY", key)
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", errors.New("API Error")
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	var rep SearchEngineResponse
	err = json.Unmarshal(body, &rep)
	if err != nil {
		return "", err
	}
	marshal, err := json.Marshal(rep)
	if err != nil {
		return "", err
	}
	return string(marshal), nil
}

func SearchEngineFunction() openai.FunctionDefinition {
	return openai.FunctionDefinition{
		Name:        "search-engine",
		Description: "查询搜索引擎获取信息",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"query": {
					Type:        jsonschema.String,
					Description: "搜索的文本内容",
				},
			},
			Required: []string{"query"},
		},
	}
}
