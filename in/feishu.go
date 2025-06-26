package in

import (
	"context"
	"errors"
	"fmt"

	"github.com/lakelink/auth-companion/out"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkcontact "github.com/larksuite/oapi-sdk-go/v3/service/contact/v3"
	"github.com/rs/zerolog/log"
)

type FeishuEventHandler struct {
	zitadelActor *out.ZitadelActor
}

func SetupFeishuEventHandler(disp *dispatcher.EventDispatcher, zitadelActor *out.ZitadelActor) *dispatcher.EventDispatcher {
	h := FeishuEventHandler{zitadelActor}
	disp = disp.OnP2UserCreatedV3(h.handleUserCreated)
	disp = disp.OnP2UserUpdatedV3(h.handleUserUpdated)
	disp = disp.OnP2UserDeletedV3(h.handleUserDeleted)

	return disp
}

func (h *FeishuEventHandler) handleUserCreated(ctx context.Context, event *larkcontact.P2UserCreatedV3) error {
	fmt.Printf("[ OnP2UserCreatedV3 access ], data: %s\n", larkcore.Prettify(event))
	_, _, err := h.zitadelActor.AddUserFromFeishu(event.Event.Object)
	return err
}

func (h *FeishuEventHandler) handleUserUpdated(ctx context.Context, event *larkcontact.P2UserUpdatedV3) error {
	fmt.Printf("[ OnP2UserUpdatedV3 access ], data: %s\n", larkcore.Prettify(event))
	status := *event.Event.Object.Status

	log.Info().Any("status", *event.Event.Object.Status).Msg("updating activated user")

	_, userId, err := h.zitadelActor.UpdateUserFromFeishu(event.Event.Object)
	if errors.Is(err, out.ErrZitadelRequireEmail) {
		log.Warn().Err(err).Str("missing", "email").Msg("incomplete feishu user profile")
		return nil
	}

	if errors.Is(err, out.ErrZitadelRequireEnName) {
		log.Warn().Err(err).Str("missing", "enName").Msg("incomplete feishu user profile")
		return nil
	}

	if errors.Is(err, out.ErrZitadelUserNotFound) {
		log.Warn().Err(err).Msg("user not found in ZITADEL, adding now")
		_, userId, err = h.zitadelActor.AddUserFromFeishu(event.Event.Object)
	}

	if err != nil {
		return err
	}

	ok := *status.IsActivated && !(*status.IsExited || *status.IsFrozen || *status.IsResigned || *status.IsUnjoin)
	if ok {
		err = h.zitadelActor.ReactivateUser(userId)
		if err != nil {
			log.Warn().Err(err).Str("userId", userId).Msg("could not reactivate user")
		}

		return nil
	} else {
		log.Info().Any("status", *event.Event.Object.Status).Msg("deactivate inactivated user")
		err = h.zitadelActor.DeactivateUser(userId)
		if err != nil {
			log.Warn().Err(err).Str("userId", userId).Msg("could not deactivate user")
		}

		return nil
	}
}

func (h *FeishuEventHandler) handleUserDeleted(ctx context.Context, event *larkcontact.P2UserDeletedV3) error {
	fmt.Printf("[ OnP2UserDeletedV3 access ], data: %s\n", larkcore.Prettify(event))
	_, err := h.zitadelActor.DeactivateUserFromFeishu(event.Event.Object)
	return err
}
