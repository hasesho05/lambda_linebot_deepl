package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"unicode"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/line/line-bot-sdk-go/linebot"
)

func UnmarshalLineRequest(data []byte) (LineRequest, error) {
	var r LineRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

type LineRequest struct {
	Events      []*linebot.Event `json:"events"`
	Destination string           `json:"destination"`
}

type Message struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Text string `json:"text"`
}

type Source struct {
	UserID string `json:"userId"`
	Type   string `json:"type"`
}

type DeepLResponse struct {
	Translations []Translated
}

type Translated struct {
	DetectedSourceLaguage string `json:"detected_source_language"`
	Text                  string `json:"text"`
}

func Handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	myLineRequest, err := UnmarshalLineRequest([]byte(request.Body))
	if err != nil {
		log.Fatal(err)
	}

	bot, err := linebot.New(
		os.Getenv("CHANNELSECRET"),
		os.Getenv("LINEACCESSTOKEN"),
	)
	if err != nil {
		log.Fatal(err)
	}

	backMsg := "文字情報を入力してください。"

	for _, e := range myLineRequest.Events {
		if e.Type == linebot.EventTypeMessage {
			switch message := e.Message.(type) {
			case *linebot.TextMessage:
				getTranslation(bot, e, message.Text)
			default:
				_, err = bot.ReplyMessage(e.ReplyToken, linebot.NewTextMessage(backMsg)).Do()
				if err != nil {
					log.Print(err)
				}
			}
		}
	}
	return events.APIGatewayProxyResponse{Body: request.Body, StatusCode: 200}, nil
}
func main() {
	lambda.Start(Handler)
}

func getTranslation(bot *linebot.Client, e *linebot.Event, text string) {
	Endpoint := "https://api-free.deepl.com/v2/translate"
	params := url.Values{}
	params.Add("auth_key", os.Getenv("ACCESSTOKEN"))
	params.Add("text", text)
	if distinctLanguage(text) {
		params.Add("source_lang", "JA")
		params.Add("target_lang", "EN")
	} else {
		params.Add("source_lang", "EN")
		params.Add("target_lang", "JA")
	}
	resp, err := http.PostForm(Endpoint, params)

	if err != nil {
		log.Fatal(err)
	}

	if err := ValidateResponse(resp); err != nil {
		log.Fatal(err)
	}
	parsed, err := ParseResponse(resp)
	if err != nil {
		log.Fatal(err)
	}
	r := []string{}
	for _, translated := range parsed.Translations {
		r = append(r, translated.Text)
	}
	result := strings.Join(r, "-")
	bot.ReplyMessage(e.ReplyToken, linebot.NewTextMessage(result)).Do()
}

func ValidateResponse(resp *http.Response) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var data map[string]interface{}
		baseErrorText := fmt.Sprintf("Invalid response [%d %s]",
			resp.StatusCode,
			http.StatusText(resp.StatusCode))
		if t, ok := KnownErrors[resp.StatusCode]; ok {
			baseErrorText += fmt.Sprintf(" %s", t)
		}
		e := json.NewDecoder(resp.Body).Decode(&data)
		if e != nil {
			return fmt.Errorf("%s", baseErrorText)
		} else {
			return fmt.Errorf("%s, %s", baseErrorText, data["message"])
		}
	}
	return nil
}

var KnownErrors = map[int]string{
	400: "Bad request. Please check error message and your parameters.",
	403: "Authorization failed. Please supply a valid auth_key parameter.",
	404: "The requested resource could not be found.",
	413: "The request size exceeds the limit.",
	414: "The request URL is too long. You can avoid this error by using a POST request instead of a GET request, and sending the parameters in the HTTP body.",
	429: "Too many requests. Please wait and resend your request.",
	456: "Quota exceeded. The character limit has been reached.",
	503: "Resource currently unavailable. Try again later.",
	529: "Too many requests. Please wait and resend your request.",
}

func ParseResponse(resp *http.Response) (DeepLResponse, error) {
	var responseJson DeepLResponse
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		err := fmt.Errorf("%s (occurred while parse response)", err.Error())
		return responseJson, err
	}
	err = json.Unmarshal(body, &responseJson)
	if err != nil {
		err := fmt.Errorf("%s (occurred while parse response)", err.Error())
		return responseJson, err
	}
	return responseJson, err
}

func distinctLanguage(text string) bool {
	for _, s := range text {
		if isHiragana(s) {
			return true
		}
		if isKatakana(s) {
			return true
		}
	}
	return false
}

func isHiragana(r rune) bool {
	return unicode.In(r, unicode.Hiragana)
}

func isKatakana(r rune) bool {
	return unicode.In(r, unicode.Katakana)
}
