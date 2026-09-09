package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func VerifyLogin(c *gin.Context) {
	var request struct {
		FlowToken string `json:"flow_token"`
		Method    string `json:"method"`
		Code      string `json:"code"`
	}
	if common.DecodeJson(c.Request.Body, &request) != nil || request.FlowToken == "" || request.Code == "" {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if request.Method == "" {
		request.Method = service.VerificationMethodTwoFA
	}
	if request.Method != service.VerificationMethodTwoFA {
		writeSecurityOperationError(c, service.ErrProofMethod)
		return
	}
	bundle, err := service.VerifyLoginCode(request.FlowToken, request.Code, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeSecurityOperationError(c, err)
		return
	}
	completeVerifiedLoginResponse(c, bundle, service.VerificationMethodTwoFA)
}

func completeVerifiedLoginResponse(c *gin.Context, bundle *service.AuthBundle, method string) {
	identity, err := service.ParseAccessToken(bundle.AccessToken)
	if err != nil {
		writeAuthSessionError(c, err)
		return
	}
	user, err := model.GetSelfUserById(identity.UserID)
	if err != nil {
		writeAuthSessionError(c, err)
		return
	}
	c.Set("login_verification_method", method)
	writeLoginResponse(c, user, bundle)
}
