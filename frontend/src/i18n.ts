import { createContext, useContext } from 'react'

export type AppLanguage = 'zh-CN' | 'en'

type MessageCatalog = {
  appTagline: string
  search: string
  manage: string
  logout: string
  admin: string
  settings: string
  preferences: string
  close: string
  genericError: string
  updateFailed: string
  uploadFailed: string
  guest: string
  owlLogoAlt: string
  dictionaryModesLabel: string
  searchQueryLabel: string
  scrollToTopLabel: string
  selectedFileCount: (count: number) => string
  dictionaryLookup: string
  lookupTitle: string
  searchPlaceholder: string
  lookupDescription: string
  lookupAction: string
  searching: string
  suggestions: string
  allDictionaries: string
  showFilters: string
  hideFilters: string
  currentScope: string
  scopeAllPublic: string
  scopeAllAccessible: string
  scopeSpecificDictionary: (name: string) => string
  recent: string
  readyTitle: string
  readyDescription: string
  noResultTitle: string
  noResultDescription: string
  bestMatch: string
  moreMatches: string
  moreEntriesFromDictionary: string
  searchOnlyThisDictionary: string
  compareAcrossDictionaries: string
  compareSameHeadword: string
  pressEnterHint: string
  resultVisibilityPublic: string
  resultVisibilityPrivate: string
  dictionaryManager: string
  managerTitle: string
  uploadedEnabled: (total: number, enabled: number) => string
  refresh: string
  mdxFile: string
  mdxFileHint: string
  chooseMdx: string
  mddResources: string
  mddHint: string
  chooseMdd: string
  uploadDictionary: string
  uploading: string
  refreshLibrary: string
  refreshItem: string
  noDictionariesYet: string
  uploadFirstDictionary: string
  enabled: string
  disabled: string
  public: string
  private: string
  disable: string
  enable: string
  delete: string
  makePublic: string
  makePrivate: string
  entries: string
  mddFiles: string
  uploadedAt: string
  owner: string
  you: string
  fileStatusOk: string
  fileStatusMissingMdx: string
  fileStatusMissingMdd: string
  fileStatusMissingAll: string
  missingFiles: string
  maintenanceTip: string
  maintenanceReportTitle: string
  discoveredCount: string
  updatedCount: string
  skippedCount: string
  failedCount: string
  guestSearchHint: string
  language: string
  login: string
  register: string
  username: string
  usernamePlaceholder: string
  password: string
  passwordPlaceholder: string
  welcomeBack: string
  createAccountTitle: string
  authDescription: string
  authHeroTitle: string
  authHeroDescription: string
  authFeatureSearch: string
  authFeatureLibrary: string
  authFeatureTheme: string
  signIn: string
  createAccount: string
  pleaseWait: string
  noDescription: string
  customFontUpload: string
  customFontPrefix: string
  readingFont: string
  theme: string
  system: string
  paper: string
  blue: string
  green: string
  retro: string
  ibm: string
  nokia: string
  gameboy: string
  blackberry: string
  nintendo: string
  dark: string
  mono_theme: string
  sans: string
  serif: string
  mono: string
  custom: string
  genericWorkspaceSubtitle: string
  workspaceSubtitle: (username: string) => string
  versionLabel: string
  deleteDictionaryConfirm: (name: string) => string
  audioPlaybackIssue: string
  audioMissingIssue: string
  dismissMessage: string
  profileLabel: string
  displayName: string
  displayNamePlaceholder: string
  uploadAvatar: string
  updateAvatar: string
  sharedControls: string
  sharedControlsDescription: string
  recentSearchLimit: string
  recentSearchLimitDescription: string
  saveRecentSearchLimit: string
  recentSearchLimitSaved: string
  mobileDictionaryFilter: string
  fontManagement: string
  fontManagementDescription: string
  fontModeLabel: string
  deleteFont: string
  profileSettings: string
  showProfileEditor: string
  hideProfileEditor: string
  saveProfile: string
  selectedAvatar: string
  systemAccess: string
  registrationGate: string
  registrationGateDescription: string
  registrationOpen: string
  registrationClosed: string
  siteFooter: string
  footerContent: string
  footerContentDescription: string
  footerExtraInfo: string
  footerExtraPlaceholder: string
  footerCopyright: string
  footerCopyrightPlaceholder: string
  saveFooterSettings: string
  fontPreview: string
  fontPreviewSample: string
  actionSucceeded: string
  settingsSaved: string
  footerSettingsSaved: string
  dictionaryUploaded: string
  dictionaryRefreshed: string
  libraryRefreshed: string
  dictionaryStatusUpdated: string
  dictionaryVisibilityUpdated: string
  dictionaryDeleted: string
  fontUploaded: string
  fontDeleted: string
  mcpAccess: string
  mcpAccessTitle: string
  mcpAccessDescription: string
  mcpToken: string
  mcpTokenPlaceholder: string
  mcpTokenConfigured: string
  mcpTokenNotConfigured: string
  saveMCPToken: string
  generateMCPToken: string
  deleteMCPToken: string
  mcpTokenSaved: string
  mcpTokenDeleted: string
  deleteMCPTokenConfirm: string
  mcpHelp: string
  mcpHelpTitle: string
  mcpHelpDescription: string
  mcpEndpoint: string
  mcpAuthorization: string
  mcpAuthorizationHeader: string
  mcpTokenExample: string
  mcpAvailableTools: string
  mcpListDictionariesHelp: string
  mcpSearchDictionaryHelp: string
  mcpHelpTokenNotice: string
}

export const messages: Record<AppLanguage, MessageCatalog> = {
  'zh-CN': {
    appTagline: '词典应用，支持 MDX / MDD',
    search: '查词',
    manage: '管理',
    logout: '退出登录',
    admin: '管理员',
    settings: '设置',
    preferences: '偏好',
    close: '关闭',
    genericError: '操作失败，请稍后再试。',
    updateFailed: '更新失败',
    uploadFailed: '上传失败',
    guest: '访客',
    owlLogoAlt: 'Owl 标志',
    dictionaryModesLabel: '词典模式',
    searchQueryLabel: '搜索关键词',
    scrollToTopLabel: '返回页面顶部',
    selectedFileCount: (count) => `已选择 ${count} 个文件`,
    dictionaryLookup: '词典查询',
    lookupTitle: '快速查单词、词组或字符。',
    searchPlaceholder: '输入 hello、ability、能力 等词汇…',
    lookupDescription: '面向日常词典使用：更快搜索、快速切换词典、更舒适的长内容阅读。',
    lookupAction: '查询',
    searching: '查询中…',
    suggestions: '搜索建议',
    allDictionaries: '全部词典',
    showFilters: '显示筛选',
    hideFilters: '收起筛选',
    currentScope: '当前范围',
    scopeAllPublic: '全部公开词典',
    scopeAllAccessible: '全部可访问词典',
    scopeSpecificDictionary: (name) => `仅限词典：${name}`,
    recent: '最近搜索',
    readyTitle: '可以开始查词了',
    readyDescription: '至少启用一本词典后开始搜索。建议先选常用主词典，再配合最近搜索使用。',
    noResultTitle: '没有找到匹配结果',
    noResultDescription: '可以尝试更短的词、不同拼写、切换到全部词典，或者刷新词典库后再试。',
    bestMatch: '最佳匹配',
    moreMatches: '更多匹配结果',
    moreEntriesFromDictionary: '来自该词典的更多匹配词条',
    searchOnlyThisDictionary: '仅在该词典中查询',
    compareAcrossDictionaries: '跨词典对比',
    compareSameHeadword: '同词跨词典对比',
    pressEnterHint: '回车使用当前输入，方向键选择建议',
    resultVisibilityPublic: '公开词典',
    resultVisibilityPrivate: '私人词典',
    dictionaryManager: '词典管理',
    managerTitle: '上传并管理你的个人词典',
    uploadedEnabled: (total, enabled) => `${total} 本已上传 · ${enabled} 本已启用`,
    refresh: '刷新',
    mdxFile: 'MDX 文件',
    mdxFileHint: '必选，包含词条正文。',
    chooseMdx: '选择 .mdx 文件',
    mddResources: 'MDD 资源',
    mddHint: '可选，包含图片、音频、CSS、字体等。',
    chooseMdd: '选择 .mdd 文件',
    uploadDictionary: '上传词典',
    uploading: '上传中…',
    refreshLibrary: '刷新词典库',
    refreshItem: '刷新词典',
    noDictionariesYet: '还没有词典',
    uploadFirstDictionary: '上传第一本 MDX 和可选 MDD 后即可开始查词。',
    enabled: '已启用',
    disabled: '已停用',
    public: '公开',
    private: '私有',
    disable: '停用',
    enable: '启用',
    delete: '删除',
    makePublic: '设为公开',
    makePrivate: '设为私有',
    entries: '词条数',
    mddFiles: 'MDD 文件',
    uploadedAt: '上传时间',
    owner: '所有者',
    you: '你',
    fileStatusOk: '文件正常',
    fileStatusMissingMdx: '缺少 MDX 主文件',
    fileStatusMissingMdd: '缺少 MDD 资源文件',
    fileStatusMissingAll: '词典文件已失效',
    missingFiles: '缺失文件',
    maintenanceTip: '如果你先上传了 MDX、后来才补上 MDD，或者挂载目录里新增了词典文件，请使用刷新词典或刷新词典库重新发现资源。',
    maintenanceReportTitle: '维护结果',
    discoveredCount: '新发现',
    updatedCount: '已更新',
    skippedCount: '已跳过',
    failedCount: '失败',
    guestSearchHint: '未登录时可查询所有已启用的公开词典。',
    language: '语言',
    login: '登录',
    register: '注册',
    username: '用户名',
    usernamePlaceholder: '输入用户名',
    password: '密码',
    passwordPlaceholder: '输入密码',
    welcomeBack: '欢迎回来',
    createAccountTitle: '创建 Owl 账户',
    authDescription: '使用用户名和密码。JWT 鉴权由后端处理。',
    authHeroTitle: '在浏览器里使用你的个人 MDX / MDD 词典。',
    authHeroDescription: '上传你自己的词典，为每个账号隔离词典库，并检索包含图片、音频和其他资源的 HTML 词条。',
    authFeatureSearch: '在已启用词典中快速模糊搜索',
    authFeatureLibrary: '每个用户独立的私人词典库',
    authFeatureTheme: '现代响应式界面，支持多主题',
    signIn: '登录',
    createAccount: '创建账户',
    pleaseWait: '请稍候…',
    noDescription: '暂无描述。',
    customFontUpload: '上传自定义字体（.ttf/.otf/.woff/.woff2）',
    customFontPrefix: '自定义字体：',
    readingFont: '阅读字体',
    theme: '主题',
    system: '跟随系统',
    paper: '纸书米白',
    blue: '蓝调',
    green: '绿调',
    retro: '复古终端',
    ibm: 'IBM 蓝白',
    nokia: 'Nokia 琥珀',
    gameboy: 'Game Boy',
    blackberry: '黑莓',
    nintendo: '任天堂',
    dark: '深色',
    mono_theme: '黑白',
    sans: '无衬线',
    serif: '衬线',
    mono: '等宽',
    custom: '自定义',
    genericWorkspaceSubtitle: '个人词典工作台',
    workspaceSubtitle: (username) => `${username} 的词典工作台`,
    versionLabel: '版本',
    deleteDictionaryConfirm: (name) => `删除词典“${name}”？`,
    audioPlaybackIssue: '当前环境暂时无法播放该音频。',
    audioMissingIssue: '当前词典资源包中不存在这条音频。',
    dismissMessage: '关闭提示',
    profileLabel: '个人资料',
    displayName: '昵称',
    displayNamePlaceholder: '输入昵称',
    uploadAvatar: '上传头像',
    updateAvatar: '更新头像',
    sharedControls: '通用控制',
    sharedControlsDescription: '个人资料、语言与主题在这里统一维护。',
    recentSearchLimit: '最近搜索数量',
    recentSearchLimitDescription: '设置最近搜索最多保留多少条；填 0 可关闭最近搜索记录。',
    saveRecentSearchLimit: '保存最近搜索设置',
    recentSearchLimitSaved: '最近搜索设置已保存',
    mobileDictionaryFilter: '词典筛选',
    fontManagement: '字体管理',
    fontManagementDescription: '字体是全局共享设置，因此单独放在管理界面维护。',
    fontModeLabel: '阅读字体模式',
    deleteFont: '删除字体',
    profileSettings: '资料设置',
    showProfileEditor: '编辑资料',
    hideProfileEditor: '收起资料设置',
    saveProfile: '保存资料',
    selectedAvatar: '已选择头像',
    systemAccess: '系统访问',
    registrationGate: '用户注册开关',
    registrationGateDescription: '控制访客登录弹窗中是否允许创建新账户，仅管理员可见。',
    registrationOpen: '允许注册',
    registrationClosed: '关闭注册',
    siteFooter: '网站页脚',
    footerContent: '页脚额外信息',
    footerContentDescription: '为空时不显示页脚；填写后访客和登录用户页面底部都会展示。',
    footerExtraInfo: '额外信息',
    footerExtraPlaceholder: '例如：本服务仅用于个人学习与词典检索',
    footerCopyright: '版权信息',
    footerCopyrightPlaceholder: '例如：© 2026 Owl Dictionary',
    saveFooterSettings: '保存页脚设置',
    fontPreview: '字体预览',
    fontPreviewSample: '快速的棕色狐狸跳过懒狗。Owl Dictionary 12345',
    actionSucceeded: '操作已完成',
    settingsSaved: '设置已保存',
    footerSettingsSaved: '页脚设置已保存',
    dictionaryUploaded: '词典已上传',
    dictionaryRefreshed: '词典已刷新',
    libraryRefreshed: '词典库已刷新',
    dictionaryStatusUpdated: '词典状态已更新',
    dictionaryVisibilityUpdated: '公开状态已更新',
    dictionaryDeleted: '词典已删除',
    fontUploaded: '字体已上传',
    fontDeleted: '字体已删除',
    mcpAccess: 'MCP 接入',
    mcpAccessTitle: 'MCP SSE 服务',
    mcpAccessDescription: '为当前用户生成或保存 MCP Token。可用范围是公开词典加你的私有词典。',
    mcpToken: 'MCP Token',
    mcpTokenPlaceholder: '输入自定义 Token，至少 16 个字符',
    mcpTokenConfigured: '已配置 Token：',
    mcpTokenNotConfigured: '尚未配置 MCP Token',
    saveMCPToken: '保存 Token',
    generateMCPToken: '生成 Token',
    deleteMCPToken: '删除 Token',
    mcpTokenSaved: 'MCP Token 已保存',
    mcpTokenDeleted: 'MCP Token 已删除',
    deleteMCPTokenConfirm: '删除当前 MCP Token？删除后使用该 Token 的客户端将无法继续连接。',
    mcpHelp: '使用说明',
    mcpHelpTitle: '如何使用 Owl MCP',
    mcpHelpDescription: 'MCP 使用 SSE 传输。每个用户使用自己的 Token，只能访问公开词典和自己上传的私有词典。',
    mcpEndpoint: 'SSE 地址',
    mcpAuthorization: '认证方式',
    mcpAuthorizationHeader: '推荐认证方式：Authorization: Bearer <你的 Token>；也可临时使用 ?token=<你的 Token>。初次 SSE 连接必须携带 Token；连接建立后，SDK 后续 POST 请求会通过 session 继续通信。',
    mcpTokenExample: '你的_MCP_TOKEN',
    mcpAvailableTools: '可用工具',
    mcpListDictionariesHelp: 'list_dictionaries：列出当前 Token 可访问的词典。',
    mcpSearchDictionaryHelp: 'search_dictionary：传入 query，可选 dictionary_id 或 dictionary_name；不传词典时按 Web 相同范围查询所有可访问词典。',
    mcpHelpTokenNotice: '生成后请立即复制 Token；出于安全考虑，之后只显示首尾提示。',

  },
  en: {
    appTagline: 'Dictionary app for MDX / MDD',
    search: 'Search',
    manage: 'Manage',
    logout: 'Logout',
    admin: 'Admin',
    settings: 'Settings',
    preferences: 'Preferences',
    close: 'Close',
    genericError: 'Something went wrong. Please try again.',
    updateFailed: 'Update failed',
    uploadFailed: 'Upload failed',
    guest: 'Guest',
    owlLogoAlt: 'Owl logo',
    dictionaryModesLabel: 'Dictionary modes',
    searchQueryLabel: 'Search query',
    scrollToTopLabel: 'Scroll to page top',
    selectedFileCount: (count) => `${count} file${count === 1 ? '' : 's'} selected`,
    dictionaryLookup: 'Dictionary Lookup',
    lookupTitle: 'Look up a word, phrase, or character instantly.',
    searchPlaceholder: 'Search words like hello, ability, 能力…',
    lookupDescription: 'Built for daily dictionary use: quick search, fast dictionary switching, and comfortable long-form entry reading.',
    lookupAction: 'Look up',
    searching: 'Searching…',
    suggestions: 'Suggestions',
    allDictionaries: 'All dictionaries',
    showFilters: 'Show filters',
    hideFilters: 'Hide filters',
    currentScope: 'Current scope',
    scopeAllPublic: 'All public dictionaries',
    scopeAllAccessible: 'All accessible dictionaries',
    scopeSpecificDictionary: (name) => `Only in ${name}`,
    recent: 'Recent',
    readyTitle: 'Ready to look up',
    readyDescription: 'Search after enabling at least one dictionary. Start with your primary dictionary and keep recent searches close by.',
    noResultTitle: 'No matching results found',
    noResultDescription: 'Try a shorter word, a different spelling, switch back to all dictionaries, or refresh the library and try again.',
    bestMatch: 'Best match',
    moreMatches: 'More matching entries',
    moreEntriesFromDictionary: 'More matching entries from this dictionary',
    searchOnlyThisDictionary: 'Search only this dictionary',
    compareAcrossDictionaries: 'Compare across dictionaries',
    compareSameHeadword: 'Compare the same headword',
    pressEnterHint: 'Press Enter to search, or use arrow keys to select a suggestion',
    resultVisibilityPublic: 'Public dictionary',
    resultVisibilityPrivate: 'Private dictionary',
    dictionaryManager: 'Dictionary Manager',
    managerTitle: 'Upload and control your personal dictionaries',
    uploadedEnabled: (total, enabled) => `${total} uploaded · ${enabled} enabled`,
    refresh: 'Refresh',
    mdxFile: 'MDX file',
    mdxFileHint: 'Required. This contains dictionary entries.',
    chooseMdx: 'Choose .mdx file',
    mddResources: 'MDD resources',
    mddHint: 'Optional. Add images, audio, CSS, fonts.',
    chooseMdd: 'Choose .mdd file(s)',
    uploadDictionary: 'Upload dictionary',
    uploading: 'Uploading…',
    refreshLibrary: 'Refresh library',
    refreshItem: 'Refresh dictionary',
    noDictionariesYet: 'No dictionaries yet',
    uploadFirstDictionary: 'Upload your first MDX and optional MDD pair to start searching.',
    enabled: 'Enabled',
    disabled: 'Disabled',
    public: 'Public',
    private: 'Private',
    disable: 'Disable',
    enable: 'Enable',
    delete: 'Delete',
    makePublic: 'Make public',
    makePrivate: 'Make private',
    entries: 'Entries',
    mddFiles: 'MDD files',
    uploadedAt: 'Uploaded',
    owner: 'Owner',
    you: 'You',
    fileStatusOk: 'Files OK',
    fileStatusMissingMdx: 'Missing MDX source',
    fileStatusMissingMdd: 'Missing MDD resources',
    fileStatusMissingAll: 'Dictionary files missing',
    missingFiles: 'Missing files',
    maintenanceTip: 'If you uploaded an MDX first and added the MDD later, or mounted new files into the library directory, use refresh to rediscover resources.',
    maintenanceReportTitle: 'Maintenance result',
    discoveredCount: 'Discovered',
    updatedCount: 'Updated',
    skippedCount: 'Skipped',
    failedCount: 'Failed',
    guestSearchHint: 'You can search all enabled public dictionaries without signing in.',
    language: 'Language',
    login: 'Login',
    register: 'Register',
    username: 'Username',
    usernamePlaceholder: 'owl-user',
    password: 'Password',
    passwordPlaceholder: '••••••••',
    welcomeBack: 'Welcome back',
    createAccountTitle: 'Create your Owl account',
    authDescription: 'Use a username and password. JWT auth is handled by the backend.',
    authHeroTitle: 'Personal MDX / MDD dictionary search, in the browser.',
    authHeroDescription: 'Upload your own dictionaries, keep them isolated per account, and search HTML entries with images, audio, and other bundled resources.',
    authFeatureSearch: 'Fast fuzzy search across enabled dictionaries',
    authFeatureLibrary: 'Private dictionary library per user',
    authFeatureTheme: 'Modern responsive UI with multiple themes',
    signIn: 'Sign in',
    createAccount: 'Create account',
    pleaseWait: 'Please wait…',
    noDescription: 'No description available.',
    customFontUpload: 'Upload custom font (.ttf/.otf/.woff/.woff2)',
    customFontPrefix: 'Custom: ',
    readingFont: 'Reading font',
    theme: 'Theme',
    system: 'system',
    paper: 'paper',
    blue: 'blue',
    green: 'green',
    retro: 'retro console',
    ibm: 'IBM blue',
    nokia: 'Nokia amber',
    gameboy: 'Game Boy',
    blackberry: 'BlackBerry',
    nintendo: 'Nintendo',
    dark: 'dark',
    mono_theme: 'mono',
    sans: 'sans',
    serif: 'serif',
    mono: 'mono',
    custom: 'custom',
    genericWorkspaceSubtitle: 'Personal dictionary workspace',
    workspaceSubtitle: (username) => `${username}'s dictionary workspace`,
    versionLabel: 'Version',
    deleteDictionaryConfirm: (name) => `Delete dictionary “${name}”?`,
    audioPlaybackIssue: 'This audio cannot be played in the current environment.',
    audioMissingIssue: 'This audio resource is missing from the current dictionary package.',
    dismissMessage: 'Dismiss',
    profileLabel: 'Profile',
    displayName: 'Display name',
    displayNamePlaceholder: 'Enter display name',
    uploadAvatar: 'Upload avatar',
    updateAvatar: 'Update avatar',
    sharedControls: 'Shared controls',
    sharedControlsDescription: 'Profile, language, and theme are maintained here for the whole app.',
    recentSearchLimit: 'Recent search count',
    recentSearchLimitDescription: 'Set how many recent searches to keep. Use 0 to disable recent search history.',
    saveRecentSearchLimit: 'Save recent search setting',
    recentSearchLimitSaved: 'Recent search setting saved',
    mobileDictionaryFilter: 'Dictionary filter',
    fontManagement: 'Font management',
    fontManagementDescription: 'Fonts are shared globally, so they are managed separately inside the dictionary workspace.',
    fontModeLabel: 'Reading font mode',
    deleteFont: 'Delete font',
    profileSettings: 'Profile settings',
    showProfileEditor: 'Edit profile',
    hideProfileEditor: 'Hide profile editor',
    saveProfile: 'Save profile',
    selectedAvatar: 'Avatar selected',
    systemAccess: 'System access',
    registrationGate: 'User registration',
    registrationGateDescription: 'Controls whether guests can create accounts from the auth dialog. Visible to admins only.',
    registrationOpen: 'Registration open',
    registrationClosed: 'Registration closed',
    siteFooter: 'Site footer',
    footerContent: 'Footer content',
    footerContentDescription: 'The footer stays hidden when both fields are empty; once saved, it appears for guests and signed-in users.',
    footerExtraInfo: 'Extra information',
    footerExtraPlaceholder: 'Example: This service is for personal study and dictionary lookup.',
    footerCopyright: 'Copyright',
    footerCopyrightPlaceholder: 'Example: © 2026 Owl Dictionary',
    saveFooterSettings: 'Save footer settings',
    fontPreview: 'Font preview',
    fontPreviewSample: 'The quick brown fox jumps over the lazy dog. Owl Dictionary 12345',
    actionSucceeded: 'Action completed',
    settingsSaved: 'Settings saved',
    footerSettingsSaved: 'Footer settings saved',
    dictionaryUploaded: 'Dictionary uploaded',
    dictionaryRefreshed: 'Dictionary refreshed',
    libraryRefreshed: 'Library refreshed',
    dictionaryStatusUpdated: 'Dictionary status updated',
    dictionaryVisibilityUpdated: 'Visibility updated',
    dictionaryDeleted: 'Dictionary deleted',
    fontUploaded: 'Font uploaded',
    fontDeleted: 'Font deleted',
    mcpAccess: 'MCP access',
    mcpAccessTitle: 'MCP SSE service',
    mcpAccessDescription: 'Generate or save an MCP token for the current user. Scope includes public dictionaries plus your private dictionaries.',
    mcpToken: 'MCP token',
    mcpTokenPlaceholder: 'Enter a custom token, at least 16 characters',
    mcpTokenConfigured: 'Token configured: ',
    mcpTokenNotConfigured: 'No MCP token configured yet',
    saveMCPToken: 'Save token',
    generateMCPToken: 'Generate token',
    deleteMCPToken: 'Delete token',
    mcpTokenSaved: 'MCP token saved',
    mcpTokenDeleted: 'MCP token deleted',
    deleteMCPTokenConfirm: 'Delete the current MCP token? Clients using this token will no longer be able to connect.',
    mcpHelp: 'Help',
    mcpHelpTitle: 'How to use Owl MCP',
    mcpHelpDescription: 'MCP uses SSE transport. Each user uses their own token and can only access public dictionaries plus private dictionaries they uploaded.',
    mcpEndpoint: 'SSE endpoint',
    mcpAuthorization: 'Authorization',
    mcpAuthorizationHeader: 'Recommended auth: Authorization: Bearer <your token>; for quick setup, ?token=<your token> is also supported. The initial SSE connection must include the token; after the connection is established, SDK POST requests continue through the session.',
    mcpTokenExample: 'YOUR_MCP_TOKEN',
    mcpAvailableTools: 'Available tools',
    mcpListDictionariesHelp: 'list_dictionaries: list dictionaries available to the current token.',
    mcpSearchDictionaryHelp: 'search_dictionary: pass query with optional dictionary_id or dictionary_name. If no dictionary is provided, it searches all dictionaries in the same accessible scope as web search.',
    mcpHelpTokenNotice: 'Copy the token immediately after generating it; for security, only a short hint is shown later.',

  },
}

type I18nContextValue = {
  language: AppLanguage
  t: MessageCatalog
}

export const I18nContext = createContext<I18nContextValue>({
  language: 'zh-CN',
  t: messages['zh-CN'],
})

export function useI18n() {
  return useContext(I18nContext)
}
