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

type str string

type myClassesEnabled struct {
	data struct {
		myClassesEnabled bool
	}
}

type ClasslinkSettings struct {
	data struct {
		customUISettings struct {
			paletteColor            string
			iconSize                string
			textSize                string
			fontBackgroundTreatment string
			textShadowDarkness      int
			backgroundType          string
			backgroundValue         string
			highContrastSelected    bool
			showFirstTime           bool
			animationEnabled        bool
			theme                   int
		}
		tenantSettings struct {
			customLogo                 str
			customText                 string
			isEnabledSSOKey            bool
			autoLaunchLimit            int
			isEnabledMyFiles           bool
			showUserAddedApps          bool
			showPasswordLocker         bool
			isEnabledNotes             bool
			isEnabledSeasonalAnimation bool
			faviconTimestamp           str
		}
		buildingSettings struct {
			loginUrl              string
			loginMessageTextColor string
			loginMessage          string
		}
		userInfo struct {
			gwsToken        string
			SourceId        string
			Profile         string
			StateName       string
			BuildingId      string
			TenantId        string
			LoginId         string
			Tenant          string
			Building        string
			DisplayName     string
			FirstName       string
			LastName        string
			ImagePath       string
			ProfileId       int
			UserId          string
			StateId         string
			Email           string
			Role            string
			FailedLogin     string // String version of failedLoginInfo
			sessionTimeout  string
			isImpersonated  bool
			impersonatedBy  interface{}
			roleLevel       string
			groupIds        []int
			failedLoginInfo struct {
				count     int
				lastLogin string
			}
			isADUser      bool
			canSetTheme   bool
			LoginSourceId int
		}
		singleSignOut struct {
			SAMLSPCode   string
			SAMLLoggedIn bool
			logoutUrls   []string
		}
	}
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
			var body = ClasslinkSettings{}
			err = json.Unmarshal(rawBody, &body)
			if err != nil {
				log.Printf("classlink script user settings unable to convert to settings object: %s", err)
				return err
			}
			body.data.tenantSettings.customText = "Tharow"
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
