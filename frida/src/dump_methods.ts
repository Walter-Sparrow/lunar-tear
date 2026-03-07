export const dumpMethods = {
  Title: {
    OnFirstStep: "Title$$OnFirstStep",
    OnPreTitle: "Title$$OnPreTitle",
    OnTitleScreen: "Title$$OnTitleScreen",
    OnApplicationVersion: "Title$$OnApplicationVersion",
    OnBanAccount: "Title$$OnBanAccount",
    OnTermOfService: "Title$$OnTermOfService",
    OnFirstDownload: "Title$$OnFirstDownload",
  },
  DarkOctoSetupper: {
    GetE: "DarkOctoSetupper$$GetE",
    StartSetup: "DarkOctoSetupper$$StartSetup",
    SetupOcto: "DarkOctoSetupper$$SetupOcto",
    CreateSetting: "DarkOctoSetupper$$CreateSetting",
  },
  WebViewConfig: {
    GetBaseUrl: "WebViewConfig$$get_BaseUrl",
  },
  NetworkConfig: {
    GetServerAddress: "NetworkConfig$$get_ServerAddress",
    GetOriginalServerAddress: "NetworkConfig$$get_OriginalServerAddress",
  },
  WebViewDialogPresenter: {
    GetWebViewServerAddress: "WebViewDialogPresenter$$GetWebViewServerAddress",
  },
  UniWebViewInterface: {
    Load: "UniWebViewInterface$$Load",
  },
  TermOfServiceDialogPresenter: {
    Setup: "TermOfServiceDialogPresenter$$Setup",
  },
  ConfigApi: {
    cctor: "Config.Api$$.cctor",
  },
} as const;
