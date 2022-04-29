x = window.siteNavigationUiProps.props;
u = u;
window.siteNavigationUiProps.props = {
  props: {
    notifications: { 
        maxAllowedEnrollments: 150, 
        unreadCount: x.notifications.unreadCount,
    },
    unreadRequestsCount: x.unreadRequestsCount,
    courses: { 
        showCreateCourse: false, 
        showJoinCourse: true 
    },
    groups: { 
        showCreateGroup: false, 
        showJoinGroup: true 
    },
    messages: { 
        canSend: true, 
        canReceive: true, 
        unreadCount: x.messages.unreadCount,
    },
    tools: {
      showUserManagement: false,
      showSchoolManagement: false,
      showImportExport: false,
      showAdvisorDashboard: false,
      showSchoolAnalytics: false,
      schoologyAdminTools: {
        showSgyManager: false,
        showDemoSchools: false,
        showServerInfo: false,
        showSgyLookup: false,
        showEmptyCache: false,
      },
    },
    amp: {
        showAssessmentTeams: false, 
        showAssessmentReports: false 
    },
    apps: {
      showAppCenterLink: false,
      userApps: x.apps.userApps,
    },
    masquerade: x.masquerade,
    grades: {
      showGradeReport: true,
      showMastery: false,
      showAttendance: false,
    },
    user: {
      language: u.language,
      languageNameNative: u.languageNameNative,
      logoutToken: u.logoutToken,
      userSessionId: u.userSessionId,
      uid: u.uid,
      name: u.name,
      profilePictureUrl: u.profilePictureUrl,
      parentUid: u.parentUid,
      parentName: u.parentName,
      parentProfilePictureUrl: u.parentProfilePictureUrl,
      blogSubscriptionStatus: false,
      buildings: u.buildings,
      linkedAccounts: u.linkedAccounts,
      childrenAccounts: u.childrenAccounts,
      isBasicInstructor: false,
      isBasicNonFacultyUser: false,
    },
    languageOptions: x.languageOptions,
    supportLink: x.supportLink,
    branding: x.branding,
    enablePdsNavigation: false,
    sentry: x.sentry,
    breadcrumbs: x.breadcrumbs,
    motuId: x.motuId,
    apiV2: { 
        rootUrl: x.apiV2.rootUrl,
    },
    isDetailLayout: false,
    termsOfUseUrl: x.termsOfUseUrl,
  },
};
