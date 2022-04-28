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


def classlink_nodeapi(response):
    # type: (Response) -> Response
    body = json.decode(response.body)
    if response.request.path == "/help":
        body['HelpLinkURL'] = "https://www.tharow.net/support"
        body['TargetEmail'] = "Contact@Tharow.net"
        body['IsEnabledContactSupport'] = 1
    if response.request.path == "/user/resourcelibrarysettings":
        body['response'][
            'ConfigureAppLibrary'] = "{\"categories\":1,\"featured\":1,\"mostpopular\":1,\"schoolname\":1,\"singlesignon\":1,\"addyourownapp\":1}"
    if response.request.path == "/user/generalsettings":
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
    if response.request.path == "/user/desktopsettings":
        body['enable_reportan_issue'] = 1
        body['is_allow_change_template'] = 1
        body['is_allow_change_wallpaper'] = 1
        body['isenabled_lplite'] = 1
        body['isenabled_reportan_issue'] = 1
        body['reportEnabled'] = 1
    if response.request.path == "/applibrary/dashboard":
        entcat = json.decode("{\"Id\":5901,\"Name\":\"Tharow\",\"TenantWide\":1}")
        body['enterprisecategories'].append(entcat)
    response.body = json.encode(body)
    return response


def classlink_io(response):
    # type: (Response) -> Response
    body = json.decode(response.body)
    if response.request.path == "/settings/v1p0/myClassesEnabled":
        body['data']['myClassesEnabled'] = True
    if response.request.path == "/settings/v1p0/settings":
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


def filter_response(response):
    # type: (Response) -> Response
    if response.request.method == "GET":
        if response.request.host == "nodeapi.classlink.com":
            return classlink_nodeapi(response)
        if response.request.host == "myapps.classlink.io":
            return classlink_io(response)

    return response
