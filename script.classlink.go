package main

import (
	"encoding/json"
	"log"
)

// ClassLink site fixer
type ClassLink Script

func (c ClassLink) Hosts() []string     { return []string{"nodeapi.classlink.com", "myapps.classlink.io"} }
func (c ClassLink) Name() string        { return "classlink" }
func (c ClassLink) Description() string { return "classlink.com fixer" }
func (c ClassLink) FilterResponse(response **Response) {
	var r = *response
	if r.Request.Request.Method == "GET" {
		switch r.Request.Request.Host {
		case "nodeapi.classlink.com":
			c.NodeAPI(response)
		case "myapps.classlink.io":
			c.MyApps(response)
		}
	}
}

func (c ClassLink) NodeAPI(response **Response) {
	var r = *response
	r.Modified = true
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

func (c ClassLink) MyApps(response **Response) {
	var r = *response
	switch r.Request.Request.URL.Path {
	case "/settings/v1p0/myClassesEnabled":
		{
			b, err := r.Body()
			if err != nil {
				return
			}
			var body = myClassesEnabled{}
			err = json.Unmarshal(b, &body)
			if err != nil {
				return
			}
			log.Print(body.data.myClassesEnabled)
			b2, err := json.Marshal(body)
			if err != nil {
				return
			}
			r.SetContent(b2, "application/json")
		}
	case "/settings/v1p0/settings":
		{
			rawBody, err := r.Body()
			if err != nil {
				log.Printf("classlink script unable to load body: %s", err)
				return
			}
			var body = ClasslinkSettings{}
			err = json.Unmarshal(rawBody, &body)
			if err != nil {
				log.Printf("classlink script user settings unable to convert to settings object: %s", err)
				return
			}
			body.data.tenantSettings.customText = "Tharow"
			bodyOut, err := json.Marshal(body)
			if err != nil {
				log.Printf("classlink script user settings unable to Marshal json")
				return
			}
			r.SetContent(bodyOut, "application/json")
		}

	}
	r.Modified = true
}
