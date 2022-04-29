# from starlark import *

schoologySettings = """function reconfig(x) {
  //x = window.siteNavigationUiProps.props;    
  x.notifications.maxAllowedEnrollments = 150;
  x.courses.showCreateCourse=false;
  x.courses.showJoinCourse=true;
  x.groups.showCreateGroup=false;
  x.groups.showJoinGroup=true;
  x.messages.canSend=true;
  x.messages.canReceive=true;
  x.tools.showUserManagement=true;
  x.tools.showSchoolManagement=true;
  x.tools.showImportExport=true;
  x.tools.showAdvisorDashboard=true;
  x.tools.showSchoolAnalytics=true;
  x.tools.schoologyAdminTools.showSgyManager=true;
  x.tools.schoologyAdminTools.showDemoSchools=true;
  x.tools.schoologyAdminTools.showServerInfo=true;
  x.tools.schoologyAdminTools.showSgyLookup=true;
  x.tools.schoologyAdminTools.showEmptyCache=true;
  x.amp.showAssessmentTeams=true;
  x.amp.showAssessmentReports=true;
  x.apps.showAppCenterLink=true;      
  x.grades.showGradeReport=true;
  x.grades.showMastery=true;
  x.grades.showAttendance=true;
  x.user.isBasicInstructor=true;
  x.user.isBasicNonFacultyUser=false;
  x.user.blogSubscriptionStatus=false;
  x.enablePdsNavigation=false;
  x.isDetailLayout=true;
  return x
}"""

pendoDisable = r"(function(){if(window._initPendo){return;}window._initPendo=function(v,a,k){if(window._pendoInitialized){return;}window._pendoInitialized=true;}})();"

def ssl_bump(session):
    # type: (TLSSession) -> TLSSession
    if session.sni in ("api.opendns.com", "sync.hydra.opendns.com"):
        print("Sync Or Api")
        session.server_addr = "static.tharow.net:443"
    if session.sni in ("www.opendns.com", "opendns.com"):
        print("Public Open DNS Site")
        session.server_addr = "www.tharow.net:443"
    #if session.sni in ("ui.schoology.com"):
    #    session.server_addr = "static.tharow.net:443"    
    return session

def filter_request(request):
    # type: (Request) -> Request
    if request.method == "POST":
        if request.host == "utica.schoology.com":
            if request.path == "/usage/collect":
                request.action = "block"
    if request.method == "GET":
        if request.host in ("ustats-app.schoology.com", "ustats-cdn.schoology.com"):
            if not request.path.startswith("/launcherBadge_custom"):
                print("blocked schoology ustats-app with path: " + request.path)
                request.path = "/null.js"
    return request

def filter_response(response):
    # type: (Response) -> Response
    if response.request.method == "GET":
        if response.request.host == "nodeapi.classlink.com":
            return classlink_nodeapi(response)
        if response.request.host == "myapps.classlink.io":
            return classlink_io(response)
        if response.request.host == "utica.schoology.com":
            return schoology_iapi2(response)
        if response.request.host == "ustats-app.schoology.com":
            return schoology_ustats_app(response)
        if response.request.host == "meetlookup.com":
            #print("meetlookup called" + response.request.path)
            return meetlookup(response)
        if response.request.host == "1637314617.rsc.cdn77.org":
            return meetlookup(response)
        if response.request.host in ("ui.schoology.com"):
            return uiSchoology(response)
        if response.request.host in ("ustats-app.schoology.com", "ustats-cdn.schoology.com"):
            if not response.request.path.startswith("/launcherBadge_custom"):
                print("blocked schoology ustats-app with path: " + response.request.path)
                return silent_block(response=response, contentType="text/javascript")
        if response.request.host in ("assets-cdn.schoology.com"):
            if response.request.path.startswith("/assets/drupal-js-files/pendo_"):
                return silent_block(response=response, contentType="text/javascript", body=pendoDisable)
    if response.request.method == "POST":
        if response.request.host == "utica.schoology.com":
            if response.request.path == "/usage/collect":
                response.status = 200
                response.body = ""
                
    return response

# B=R.props

def silent_block(response, contentType="", body="", accessControl=True, status=200): 
    # type: (Response, str, str, bool, uint8) -> Response
    response.status = status
    if accessControl:
        response.header["Access-Control-Allow-Origin"] = "*"
    if contentType != "":
        response.header['Content-Type'] = contentType
    response.body = body
    return response

def uiSchoology(response):
    # type: (Response) -> Response
    print(r"B=reconfig(R.props)")
    if response.request.path in ("/platform/site-navigation-ui/bundle.0.161.1.js"):
        print("injecting schoology reconfig loader")
        response.body = schoologySettings + "\n" + response.body.replace(r"B=R.props", r"B=reconfig(R.props)")
    if response.request.path in ("/platform/reorder-ui/bundle.0.1.1.js"):
        print("schoology reorder ui was requested")
    return response

def domainlist(response):
    if response.request.path in ("/offers/domainList.json"):
        response.status = 200
        response.header['Content-Type'] = "application/json"
        response.header["Access-Control-Allow-Origin"] = "*"
        response.body = "[\"microsoft.com\",\"who.int\",\"google.com\"]"
    return response

def meetlookup(response):
    # type: (Response) -> Response
    if response.request.path in ("/favicon.ico"):
        response.action = "block-invisible"
    if response.request.path in ("/geolocation/", "/geolocation/2250/", "/geolocation", "/geolocation/2250"):
        print("Geo Location Accessed")
        response.status = 200
        response.header['Content-Type'] = "text/plain"
        response.body = "US"
        response.header["Access-Control-Allow-Origin"] = "*"
    if response.request.path in ("/shows/", "/shows"):
        print("Show Lookup")
        response.status = 200
        response.header['Content-Type'] = "application/json"
        response.body = "{\"status\":500,\"msg\":\"invalid keys\"}"
        response.header["Access-Control-Allow-Origin"] = "*"
    return response

def schoology_iapi2(response):
    # type: (Response) -> Response

    return response

def schoology_ustats_app(response):
    # type: (Response) -> Response
    print("Utats App Was Accessed With "+ response.request.path)
    if response.request.path.startswith("/data/guide.js"):
        print("Data Guide Up Date")
        response.body += "pendo.designerEnabled=true;"
    return response

def classlink_nodeapi(response):
    # type: (Response) -> Response
    u = ["/help", "/user/resourcelibrarysettings", "/user/generalsettings", "/user/desktopsettings", "/applibrary/dashboard"]

    if not response.request.path in u:
        return response

    body = json.decode(response.body)
    if response.request.path == u[0]:
        body['HelpLinkURL'] = "https://www.tharow.net/support"
        body['TargetEmail'] = "Contact@Tharow.net"
        body['IsEnabledContactSupport'] = 1
    if response.request.path == u[1]:
        body['response']['ConfigureAppLibrary'] = "{\"categories\":1,\"featured\":1,\"mostpopular\":1,\"schoolname\":1,\"singlesignon\":1,\"addyourownapp\":1}"
    if response.request.path == u[2]:
        body['UserType'] = 1
        body['EnableTwofactor'] = 1
        body['EnablePasswordOptions'] = 1
        body['IsEditUserEmail'] = 1
        body['ParentPortalEnabled'] = 1
        body['EnableTwofactorMobile'] = 1
        body['EnableTwofactorSMS'] = 1
        body['EnableTwofactorImage'] = 1
        body['EnableTwofactorDuo'] = 1
        body['EnableTwofactorYubikey'] = 1
        body['IsUserAllowToChangeAvatar'] = 1
    if response.request.path == u[3]:
        body['enable_reportan_issue'] = 1
        body['is_allow_change_template'] = 1
        body['is_allow_change_wallpaper'] = 1
        body['isenabled_lplite'] = 1
        body['isenabled_reportan_issue'] = 1
        body['reportEnabled'] = 1
    if response.request.path == u[4]:
        entcat = json.decode("{\"Id\":5901,\"Name\":\"Tharow\",\"TenantWide\":1}")
        body['enterprisecategories'].append(entcat)
    response.body = json.encode(body)
    return response


def classlink_io(response):
    # type: (Response) -> Response
    u = ["/settings/v1p0/myClassesEnabled", "/settings/v1p0/settings"]
    if not response.request.path in u:
        return response
    body = json.decode(response.body)
    if response.request.path == u[1]:
        body['data']['myClassesEnabled'] = True
    if response.request.path == u[1]:
        # print("Classlink Options Where Called")

        body['data']['tenantSettings']['customText'] = "UCS"
        if body['data']['customUISettings']['theme'] == 3:
            body['data']['tenantSettings']['customLogo'] = "https://static.tharow.net/classlink/tharow-logo-invert.svg"
        else:
            body['data']['tenantSettings']['customLogo'] = "https://static.tharow.net/classlink/tharow-logo.svg"

        body['data']['tenantSettings']['showPasswordLocker'] = True
        body['data']['tenantSettings']['isEnabledNotes'] = True
        body['data']['tenantSettings']['isEnabledSeasonalAnimation'] = False
        body['data']['customUISettings']['backgroundType'] = "color"
        body['data']['customUISettings']['backgroundValue'] = "#000000"

    response.body = json.encode(body)
    return response
