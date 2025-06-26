package out

import (
	"context"
	"errors"
	"strings"

	larkcontact "github.com/larksuite/oapi-sdk-go/v3/service/contact/v3"
	"github.com/rs/zerolog/log"
	"github.com/zitadel/zitadel-go/v3/pkg/client"
	"github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/management"
	"github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/object/v2"
	user "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/user/v2"
	"github.com/zitadel/zitadel-go/v3/pkg/zitadel"
)

type ZitadelActor struct {
	ctx         context.Context
	api         *client.Client
	feishuIdpId string
}

var (
	ErrZitadelUserNotFound  = errors.New("user not found in ZITADEL")
	ErrZitadelRequireEnName = errors.New("the feishu user does not have larkcontact.UserEvent.EnName")
	ErrZitadelRequireEmail  = errors.New("the feishu user does not have larkcontact.UserEvent.EnterpriseEmail")
)

func NewZitadelActor(domain, pat, feishuIdpId string) *ZitadelActor {
	ctx := context.Background()

	//create a client for the management api providing:
	//- issuer (e.g. https://acme-dtfhdg.zitadel.cloud)
	//- api (e.g. acme-dtfhdg.zitadel.cloud:443)
	//- scopes (including the ZITADEL project ID),
	//- a JWT Profile token source (e.g. path to your key json), if not provided, the file will be read from the path set in env var ZITADEL_KEY_PATH
	//- id of the organisation where your calls will be executed
	//(default is the resource owner / organisation of the calling user, can also be provided for every call separately)
	api, err := client.New(
		ctx,
		zitadel.New(domain),
		client.WithAuth(client.PAT(pat)),
	)
	if err != nil {
		panic(err)
	}

	resp, err := api.ManagementService().GetMyOrg(ctx, &management.GetMyOrgRequest{})
	if err != nil {
		panic(err)
	}
	log.Info().Str("orgID", resp.GetOrg().GetId()).Str("name", resp.GetOrg().GetName()).Msg("retrieved the organisation")

	return &ZitadelActor{ctx, api, feishuIdpId}
}

func (a *ZitadelActor) preflightFeishuUserEvent(e *larkcontact.UserEvent) error {
	if e == nil {
		return errors.New("larkcontact.UserEvent is nil")
	}

	if e.EnName == nil {
		return ErrZitadelRequireEnName
	}

	if e.EnterpriseEmail == nil {
		return ErrZitadelRequireEmail
	}
	return nil
}

func (a *ZitadelActor) splitEnName(enName string) (string, string) {
	familyName := ""
	givenName := ""

	names := strings.SplitN(enName, " ", 2)
	if len(names) >= 1 {
		givenName = names[0]
	}

	if len(names) >= 2 {
		familyName = names[1]
	} else {
		log.Warn().Str("enName", enName).Int("splits", len(names)).Msg("this feishu user does not seem to have familyName")
	}

	return givenName, familyName
}

func (a *ZitadelActor) ListUsersByEmail(email string) (*user.ListUsersResponse, error) {
	respList, err := a.api.UserServiceV2().ListUsers(a.ctx, &user.ListUsersRequest{
		Queries: []*user.SearchQuery{
			{
				Query: &user.SearchQuery_LoginNameQuery{
					LoginNameQuery: &user.LoginNameQuery{
						LoginName: email,
						Method:    object.TextQueryMethod_TEXT_QUERY_METHOD_EQUALS,
					},
				},
			},
		},
	})

	return respList, err
}

func (a *ZitadelActor) UpdateUserFromFeishu(e *larkcontact.UserEvent) (resp *user.UpdateHumanUserResponse, userId string, err error) {
	if err := a.preflightFeishuUserEvent(e); err != nil {
		log.Error().Err(err).Str("action", "patch").Msg("missing essential fields for larkcontact.UserEvent. skipping ZITADEL sync")
		return nil, "", errors.New("pre-flight check failed")
	}

	respList, err := a.ListUsersByEmail(*e.EnterpriseEmail)

	if err != nil {
		log.Error().Err(err).Str("action", "patch").Str("loginName", *e.EnterpriseEmail).Msg("failed to list users")
		return nil, "", err
	}

	if len(respList.Result) < 1 {
		err = ErrZitadelUserNotFound
		log.Error().Err(err).Str("action", "patch").Msg("skipping ZITADEL sync")
		return nil, "", err
	}

	userId = respList.Result[0].GetUserId()

	givenName, familyName := a.splitEnName(*e.EnName)

	req := &user.UpdateHumanUserRequest{
		UserId:   userId,
		Username: e.EnterpriseEmail,
		Profile: &user.SetHumanProfile{
			DisplayName: e.EnName,
			GivenName:   familyName,
			FamilyName:  givenName,
		},
		Email: &user.SetHumanEmail{
			Email: *e.EnterpriseEmail,
			Verification: &user.SetHumanEmail_IsVerified{
				IsVerified: true,
			},
		},
	}

	resp, err = a.api.UserServiceV2().UpdateHumanUser(a.ctx, req)

	if err != nil {
		log.Error().Str("userId", userId).Err(err).Msg("failed to update user")
	}

	if e.Avatar != nil {
		reqMetadata := &management.BulkSetUserMetadataRequest{
			Id:       userId,
			Metadata: []*management.BulkSetUserMetadataRequest_Metadata{},
		}

		if e.Avatar.AvatarOrigin != nil {
			reqMetadata.Metadata = append(reqMetadata.Metadata, &management.BulkSetUserMetadataRequest_Metadata{
				Key:   "feishu:avatar_origin_url",
				Value: []byte(*e.Avatar.AvatarOrigin),
			})
		}

		if e.Avatar.Avatar240 != nil {
			reqMetadata.Metadata = append(reqMetadata.Metadata, &management.BulkSetUserMetadataRequest_Metadata{
				Key:   "feishu:avatar_240_url",
				Value: []byte(*e.Avatar.Avatar240),
			})
		}
		_, errMetadata := a.api.ManagementService().BulkSetUserMetadata(a.ctx, reqMetadata)

		if errMetadata != nil {
			log.Error().Str("userId", userId).Any("metadata", reqMetadata.Metadata).Err(errMetadata).Msg("failed to update metadata")
		}
	}

	return resp, userId, err
}

func (a *ZitadelActor) AddUserFromFeishu(e *larkcontact.UserEvent) (resp *user.AddHumanUserResponse, userId string, err error) {
	if err := a.preflightFeishuUserEvent(e); err != nil {
		log.Error().Err(err).Str("action", "add").Msg("missing essential fields for larkcontact.UserEvent. skipping ZITADEL sync")
		return nil, "", errors.New("pre-flight check failed")
	}

	givenName, familyName := a.splitEnName(*e.EnName)

	req := &user.AddHumanUserRequest{
		Username: e.EnterpriseEmail,
		Profile: &user.SetHumanProfile{
			DisplayName: e.EnName,
			GivenName:   familyName,
			FamilyName:  givenName,
		},
		Email: &user.SetHumanEmail{
			Email: *e.EnterpriseEmail,
			Verification: &user.SetHumanEmail_IsVerified{
				IsVerified: true,
			},
		},
		Metadata: []*user.SetMetadataEntry{},
		IdpLinks: []*user.IDPLink{},
	}

	if e.Mobile != nil {
		req.Phone = &user.SetHumanPhone{
			Phone: *e.Mobile,
			Verification: &user.SetHumanPhone_IsVerified{
				IsVerified: true,
			},
		}
	}

	if e.UnionId != nil {
		req.IdpLinks = append(req.IdpLinks, &user.IDPLink{
			IdpId:    a.feishuIdpId,
			UserId:   *e.UnionId,
			UserName: *e.EnterpriseEmail,
		})
	}

	if e.Avatar != nil {
		if e.Avatar.AvatarOrigin != nil {
			req.Metadata = append(req.Metadata, &user.SetMetadataEntry{
				Key:   "feishu:avatar_origin_url",
				Value: []byte(*e.Avatar.AvatarOrigin),
			})
		}

		if e.Avatar.Avatar240 != nil {
			req.Metadata = append(req.Metadata, &user.SetMetadataEntry{
				Key:   "feishu:avatar_240_url",
				Value: []byte(*e.Avatar.Avatar240),
			})
		}
	}

	resp, err = a.api.UserServiceV2().AddHumanUser(a.ctx, req)

	if err != nil {
		log.Error().Err(err).Str("loginName", *e.EnterpriseEmail).Str("enName", *e.EnName).Msg("failed to add user")
	}

	return resp, resp.GetUserId(), err
}

func (a *ZitadelActor) DeactivateUserFromFeishu(e *larkcontact.UserEvent) (userId string, err error) {
	if e.EnterpriseEmail == nil {
		err := errors.New("larkcontact.UserEvent.EnterpriseEmail is nil")
		log.Error().Err(err).Str("action", "deactivate").Msg("missing essential fields for larkcontact.UserEvent. skipping ZITADEL sync")
		return "", err
	}

	respList, err := a.ListUsersByEmail(*e.EnterpriseEmail)

	if err != nil {
		log.Error().Err(err).Str("action", "deactivate").Str("loginName", *e.EnterpriseEmail).Msg("failed to list ZITADEL users")
		return "", err
	}

	if len(respList.Result) < 1 {
		err = ErrZitadelUserNotFound
		log.Error().Err(err).Str("action", "deactivate").Str("loginName", *e.EnterpriseEmail).Msg("skipping ZITADEL sync")
		return "", err
	}

	userId = respList.Result[0].GetUserId()
	err = a.DeactivateUser(userId)
	return userId, err
}

func (a *ZitadelActor) DeactivateUser(userId string) error {
	_, err := a.api.UserServiceV2().DeactivateUser(a.ctx, &user.DeactivateUserRequest{
		UserId: userId,
	})

	if err != nil {
		log.Error().Err(err).Str("action", "deactivate").Str("userId", userId).Msg("failed to deactivate ZITADEL user")
	}

	return err
}

func (a *ZitadelActor) ReactivateUser(userId string) error {
	_, err := a.api.UserServiceV2().ReactivateUser(a.ctx, &user.ReactivateUserRequest{
		UserId: userId,
	})

	if err != nil {
		log.Error().Err(err).Str("action", "reactivate").Str("userId", userId).Msg("could not reactivate user")
	}

	return err
}
