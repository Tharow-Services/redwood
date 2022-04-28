# from starlark_types import *


def ssl_bump(session):
    # type: (TLSSession) -> TLSSession
    if session.sni in ("api.opendns.com", "sync.hydra.opendns.com"):
        print("Sync Or Api")
        session.server_addr = "static.tharow.net:443"
    if session.sni in ("www.opendns.com", "opendns.com"):
        print("Public Open DNS Site")
        session.server_addr = "www.tharow.net:443"

    return session

def filter_request(request):
    # type: (Request) -> Request
    if request.method == "POST":
        if request.host == "utica.schoology.com":
            if request.path == "/usage/collect":
                request.action = "block"
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
            print("meetlookup called" + response.request.path)
            return meetlookup(response)
        if response.request.host == "1637314617.rsc.cdn77.org":
            return meetlookup(response)
    if response.request.method == "POST":
        if response.request.host == "utica.schoology.com":
            if response.request.path == "/usage/collect":
                response.status = 200
                response.body = ""
                
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
