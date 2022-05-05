package main

import (
	"encoding/json"
	"fmt"
	"log"
)

// ClassLink site fixer
type ClassLink struct{}

func (c ClassLink) NodeAPI(response *Response) {
	response.Modified = true
}

type myClassesEnabled struct {
	data struct {
		myClassesEnabled bool
	}
}

type ClassLinkSettingsJson struct {
	Data struct {
		UISettings struct {
			PaletteColor            string `json:"paletteColor"`
			IconSize                string `json:"iconSize"`
			TextSize                string `json:"textSize"`
			FontBackgroundTreatment string `json:"fontBackgroundTreatment"`
			TextShadowDarkness      int    `json:"textShadowDarkness"`
			BackgroundType          string `json:"backgroundType"`
			BackgroundValue         string `json:"backgroundValue"`
			HighContrastSelected    bool   `json:"highContrastSelected"`
			ShowFirstTime           bool   `json:"showFirstTime"`
			AnimationEnabled        bool   `json:"animationEnabled"`
			Theme                   int    `json:"theme"`
		} `json:"customUISettings"`
		Tenant struct {
			Logo              *string `json:"customLogo"`
			Text              string  `json:"customText"`
			SSOKey            bool    `json:"isEnabledSSOKey"`
			AutoLaunchLimit   int     `json:"autoLaunchLimit"`
			MyFiles           bool    `json:"isEnabledMyFiles"`
			UserAddedApps     bool    `json:"showUserAddedApps"`
			PasswordLocker    bool    `json:"showPasswordLocker"`
			Notes             bool    `json:"isEnabledNotes"`
			SeasonalAnimation bool    `json:"isEnabledSeasonalAnimation"`
			FaviconTime       *string `json:"faviconTimestamp"`
		} `json:"tenantSettings"`
		Building struct {
			LoginUrl              string `json:"loginUrl"`
			LoginMessageTextColor string `json:"loginMessageTextColor"`
			LoginMessage          string `json:"loginMessage"`
		} `json:"buildingSettings"`
		User struct {
			GwsToken        string      `json:"gwsToken"`
			SourceId        string      `json:"SourceId"`
			Profile         string      `json:"Profile"`
			StateName       string      `json:"StateName"`
			BuildingId      string      `json:"BuildingId"`
			TenantId        string      `json:"TenantId"`
			LoginId         string      `json:"LoginId"`
			Tenant          string      `json:"Tenant"`
			Building        string      `json:"Building"`
			DisplayName     string      `json:"DisplayName"`
			FirstName       string      `json:"FirstName"`
			LastName        string      `json:"LastName"`
			ImagePath       string      `json:"ImagePath"`
			ProfileId       int         `json:"ProfileId"`
			UserId          string      `json:"UserId"`
			StateId         string      `json:"StateId"`
			Email           string      `json:"Email"`
			Role            string      `json:"Role"`
			FailedLogin     string      `json:"FailedLogin"` // String version of failedLoginInfo
			SessionTimeout  string      `json:"sessionTimeout"`
			IsImpersonated  bool        `json:"isImpersonated"`
			ImpersonatedBy  interface{} `json:"impersonatedBy"`
			RoleLevel       string      `json:"roleLevel"`
			GroupIds        []int       `json:"groupIds"`
			FailedLoginInfo struct {
				Count     int    `json:"count"`
				LastLogin string `json:"lastLogin"`
			} `json:"failedLoginInfo"`
			ADUser        bool `json:"isADUser"`
			CanSetTheme   bool `json:"canSetTheme"`
			LoginSourceId int  `json:"LoginSourceId"`
		} `json:"userInfo"`
		SSO struct {
			SAMLSPCode   string   `json:"SAMLSPCode"`
			SAMLLoggedIn bool     `json:"SAMLLoggedIn"`
			LogoutUrls   []string `json:"logoutUrls"`
		} `json:"singleSignOut"`
	} `json:"data"`
}

func (c ClassLink) MyApps(response *Response) error {
	switch response.Request.Request.URL.Path {
	case "/settings/v1p0/myClassesEnabled":
		{
			b, err := response.Body()
			if err != nil {
				return err
			}
			var body = myClassesEnabled{}
			err = json.Unmarshal(b, &body)
			if err != nil {
				return fmt.Errorf("unmarshal error: %s", err)
			}
			log.Print(body.data.myClassesEnabled)
			b2, err := json.Marshal(body)
			if err != nil {
				return err
			}
			response.SetContent(b2, "application/json")
		}
	case "/settings/v1p0/settings":
		{
			rawBody, err := response.Body()
			if err != nil {
				log.Printf("classlink script unable to load body: %s", err)
				return err
			}
			var body = ClassLinkSettingsJson{}
			err = json.Unmarshal(rawBody, &body)
			if err != nil {
				log.Printf("classlink script user settings unable to convert to settings object: %s", err)
				return err
			}
			body.Data.Tenant.MyFiles = false
			body.Data.Tenant.Notes = true
			body.Data.Tenant.Text = "UCSv2"
			bodyOut, err := json.Marshal(body)
			if err != nil {
				log.Printf("classlink script user settings unable to Marshal json")
				return err
			}
			response.SetContent(bodyOut, "application/json")
		}

	}
	response.Modified = true
	return nil
}
