package out

import (
	"context"
	"encoding/json"
	"fmt"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type FeishuActor struct {
	c *lark.Client
}

func NewFeishuActor() *FeishuActor {
	a := FeishuActor{}
	app_id, app_secret := viper.GetString("feishu.app_id"), viper.GetString("feishu.app_secret")
	a.c = lark.NewClient(app_id, app_secret)

	return &a
}

func (a *FeishuActor) SendTextMessage(receiveIdType, receiveId string, text string) error {

	b, err := json.Marshal(map[string]string{
		"text": text,
	})

	if err != nil {
		return err
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIdType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(receiveId).
			MsgType(`text`).
			Content(string(b)).
			Build()).
		Build()

	resp, err := a.c.Im.V1.Message.Create(context.Background(), req)

	if err != nil {
		return err
	}

	if !resp.Success() {
		return fmt.Errorf("logId: %s, error response: \n%s", resp.RequestId(), larkcore.Prettify(resp.CodeError))
	}

	log.Info().Str("receive_id_type", receiveIdType).Str("receive_id", receiveId).Msg("feishu message sent")

	return nil
}
