// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	l4g "github.com/alecthomas/log4go"
	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"net/http"

	"github.com/mattermost/mattermost-server/einterfaces"
	"github.com/mattermost/mattermost-server/model"
)

const (
	MODE_DEV        = "dev"
	MODE_BETA       = "beta"
	MODE_PROD       = "prod"
	LOG_ROTATE_SIZE = 10000
	LOG_FILENAME    = "mattermost.log"
)

var originalDisableDebugLvl l4g.Level = l4g.DEBUG
var siteURL = ""

func GetSiteURL() string {
	return siteURL
}

func SetSiteURL(url string) {
	siteURL = strings.TrimRight(url, "/")
}

// FindConfigFile attempts to find an existing configuration file. fileName can be an absolute or
// relative path or name such as "/opt/mattermost/config.json" or simply "config.json". An empty
// string is returned if no configuration is found.
func FindConfigFile(fileName string) (path string) {
	if filepath.IsAbs(fileName) {
		if _, err := os.Stat(fileName); err == nil {
			return fileName
		}
	} else {
		for _, dir := range []string{"./config", "../config", "../../config", "."} {
			path, _ := filepath.Abs(filepath.Join(dir, fileName))
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	return ""
}

func FindDir(dir string) (string, bool) {
	fileName := "."
	found := false
	if _, err := os.Stat("./" + dir + "/"); err == nil {
		fileName, _ = filepath.Abs("./" + dir + "/")
		found = true
	} else if _, err := os.Stat("../" + dir + "/"); err == nil {
		fileName, _ = filepath.Abs("../" + dir + "/")
		found = true
	} else if _, err := os.Stat("../../" + dir + "/"); err == nil {
		fileName, _ = filepath.Abs("../../" + dir + "/")
		found = true
	}

	return fileName + "/", found
}

func DisableDebugLogForTest() {
	if l4g.Global["stdout"] != nil {
		originalDisableDebugLvl = l4g.Global["stdout"].Level
		l4g.Global["stdout"].Level = l4g.ERROR
	}
}

func EnableDebugLogForTest() {
	if l4g.Global["stdout"] != nil {
		l4g.Global["stdout"].Level = originalDisableDebugLvl
	}
}

func ConfigureCmdLineLog() {
	ls := model.LogSettings{}
	ls.EnableConsole = true
	ls.ConsoleLevel = "WARN"
	ConfigureLog(&ls)
}

// TODO: this code initializes console and file logging. It will eventually be replaced by JSON logging in logger/logger.go
// See PLT-3893 for more information
func ConfigureLog(s *model.LogSettings) {

	l4g.Close()

	if s.EnableConsole {
		level := l4g.DEBUG
		if s.ConsoleLevel == "INFO" {
			level = l4g.INFO
		} else if s.ConsoleLevel == "WARN" {
			level = l4g.WARNING
		} else if s.ConsoleLevel == "ERROR" {
			level = l4g.ERROR
		}

		lw := l4g.NewConsoleLogWriter()
		lw.SetFormat("[%D %T] [%L] %M")
		l4g.AddFilter("stdout", level, lw)
	}

	if s.EnableFile {

		var fileFormat = s.FileFormat

		if fileFormat == "" {
			fileFormat = "[%D %T] [%L] %M"
		}

		level := l4g.DEBUG
		if s.FileLevel == "INFO" {
			level = l4g.INFO
		} else if s.FileLevel == "WARN" {
			level = l4g.WARNING
		} else if s.FileLevel == "ERROR" {
			level = l4g.ERROR
		}

		flw := l4g.NewFileLogWriter(GetLogFileLocation(s.FileLocation), false)
		flw.SetFormat(fileFormat)
		flw.SetRotate(true)
		flw.SetRotateLines(LOG_ROTATE_SIZE)
		l4g.AddFilter("file", level, flw)
	}
}

func GetLogFileLocation(fileLocation string) string {
	if fileLocation == "" {
		logDir, _ := FindDir("logs")
		return logDir + LOG_FILENAME
	} else {
		return path.Join(fileLocation, LOG_FILENAME)
	}
}

func SaveConfig(fileName string, config *model.Config) *model.AppError {
	b, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return model.NewAppError("SaveConfig", "utils.config.save_config.saving.app_error",
			map[string]interface{}{"Filename": fileName}, err.Error(), http.StatusBadRequest)
	}

	err = ioutil.WriteFile(fileName, b, 0644)
	if err != nil {
		return model.NewAppError("SaveConfig", "utils.config.save_config.saving.app_error",
			map[string]interface{}{"Filename": fileName}, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

type ConfigWatcher struct {
	watcher *fsnotify.Watcher
	close   chan struct{}
	closed  chan struct{}
}

func NewConfigWatcher(cfgFileName string, f func()) (*ConfigWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create config watcher for file: "+cfgFileName)
	}

	configFile := filepath.Clean(cfgFileName)
	configDir, _ := filepath.Split(configFile)
	watcher.Add(configDir)

	ret := &ConfigWatcher{
		watcher: watcher,
		close:   make(chan struct{}),
		closed:  make(chan struct{}),
	}

	go func() {
		defer close(ret.closed)
		defer watcher.Close()

		for {
			select {
			case event := <-watcher.Events:
				// we only care about the config file
				if filepath.Clean(event.Name) == configFile {
					if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
						l4g.Info(fmt.Sprintf("Config file watcher detected a change reloading %v", cfgFileName))

						if _, configReadErr := ReadConfigFile(cfgFileName, true); configReadErr == nil {
							f()
						} else {
							l4g.Error(fmt.Sprintf("Failed to read while watching config file at %v with err=%v", cfgFileName, configReadErr.Error()))
						}
					}
				}
			case err := <-watcher.Errors:
				l4g.Error(fmt.Sprintf("Failed while watching config file at %v with err=%v", cfgFileName, err.Error()))
			case <-ret.close:
				return
			}
		}
	}()

	return ret, nil
}

func (w *ConfigWatcher) Close() {
	close(w.close)
	<-w.closed
}

// ReadConfig reads and parses the given configuration.
func ReadConfig(r io.Reader, allowEnvironmentOverrides bool) (*model.Config, error) {
	v := viper.New()

	if allowEnvironmentOverrides {
		v.SetEnvPrefix("mm")
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		v.AutomaticEnv()
	}

	v.SetConfigType("json")
	if err := v.ReadConfig(r); err != nil {
		return nil, err
	}

	var config model.Config
	unmarshalErr := v.Unmarshal(&config)
	if unmarshalErr == nil {
		// https://github.com/spf13/viper/issues/324
		// https://github.com/spf13/viper/issues/348
		config.PluginSettings = model.PluginSettings{}
		unmarshalErr = v.UnmarshalKey("pluginsettings", &config.PluginSettings)
	}
	return &config, unmarshalErr
}

// ReadConfigFile reads and parses the configuration at the given file path.
func ReadConfigFile(path string, allowEnvironmentOverrides bool) (*model.Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ReadConfig(f, allowEnvironmentOverrides)
}

// EnsureConfigFile will attempt to locate a config file with the given name. If it does not exist,
// it will attempt to locate a default config file, and copy it to a file named fileName in the same
// directory. In either case, the config file path is returned.
func EnsureConfigFile(fileName string) (string, error) {
	if configFile := FindConfigFile(fileName); configFile != "" {
		return configFile, nil
	}
	if defaultPath := FindConfigFile("default.json"); defaultPath != "" {
		destPath := filepath.Join(filepath.Dir(defaultPath), fileName)
		src, err := os.Open(defaultPath)
		if err != nil {
			return "", err
		}
		defer src.Close()
		dest, err := os.OpenFile(destPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return "", err
		}
		defer dest.Close()
		if _, err := io.Copy(dest, src); err == nil {
			return destPath, nil
		}
	}
	return "", fmt.Errorf("no config file found")
}

// LoadConfig will try to search around for the corresponding config file.  It will search
// /tmp/fileName then attempt ./config/fileName, then ../config/fileName and last it will look at
// fileName.
func LoadConfig(fileName string) (config *model.Config, configPath string, appErr *model.AppError) {
	if fileName != filepath.Base(fileName) {
		configPath = fileName
	} else {
		if path, err := EnsureConfigFile(fileName); err != nil {
			appErr = model.NewAppError("LoadConfig", "utils.config.load_config.opening.panic", map[string]interface{}{"Filename": fileName, "Error": err.Error()}, "", 0)
			return
		} else {
			configPath = path
		}
	}

	config, err := ReadConfigFile(configPath, true)
	if err != nil {
		appErr = model.NewAppError("LoadConfig", "utils.config.load_config.decoding.panic", map[string]interface{}{"Filename": fileName, "Error": err.Error()}, "", 0)
		return
	}

	needSave := len(config.SqlSettings.AtRestEncryptKey) == 0 || len(*config.FileSettings.PublicLinkSalt) == 0 ||
		len(config.EmailSettings.InviteSalt) == 0

	config.SetDefaults()

	if err := config.IsValid(); err != nil {
		return nil, "", err
	}

	if needSave {
		if err := SaveConfig(configPath, config); err != nil {
			l4g.Warn(err.Error())
		}
	}

	if err := ValidateLocales(config); err != nil {
		if err := SaveConfig(configPath, config); err != nil {
			l4g.Warn(err.Error())
		}
	}

	if *config.FileSettings.DriverName == model.IMAGE_DRIVER_LOCAL {
		dir := config.FileSettings.Directory
		if len(dir) > 0 && dir[len(dir)-1:] != "/" {
			config.FileSettings.Directory += "/"
		}
	}

	return config, configPath, nil
}

func GenerateClientConfig(c *model.Config, diagnosticId string) map[string]string {
	props := make(map[string]string)

	props["Version"] = model.CurrentVersion
	props["BuildNumber"] = model.BuildNumber
	props["BuildDate"] = model.BuildDate
	props["BuildHash"] = model.BuildHash
	props["BuildHashEnterprise"] = model.BuildHashEnterprise
	props["BuildEnterpriseReady"] = model.BuildEnterpriseReady

	props["SiteURL"] = strings.TrimRight(*c.ServiceSettings.SiteURL, "/")
	props["SiteName"] = c.TeamSettings.SiteName
	props["EnableTeamCreation"] = strconv.FormatBool(c.TeamSettings.EnableTeamCreation)
	props["EnableUserCreation"] = strconv.FormatBool(c.TeamSettings.EnableUserCreation)
	props["EnableOpenServer"] = strconv.FormatBool(*c.TeamSettings.EnableOpenServer)
	props["RestrictDirectMessage"] = *c.TeamSettings.RestrictDirectMessage
	props["RestrictTeamInvite"] = *c.TeamSettings.RestrictTeamInvite
	props["RestrictPublicChannelCreation"] = *c.TeamSettings.RestrictPublicChannelCreation
	props["RestrictPrivateChannelCreation"] = *c.TeamSettings.RestrictPrivateChannelCreation
	props["RestrictPublicChannelManagement"] = *c.TeamSettings.RestrictPublicChannelManagement
	props["RestrictPrivateChannelManagement"] = *c.TeamSettings.RestrictPrivateChannelManagement
	props["RestrictPublicChannelDeletion"] = *c.TeamSettings.RestrictPublicChannelDeletion
	props["RestrictPrivateChannelDeletion"] = *c.TeamSettings.RestrictPrivateChannelDeletion
	props["RestrictPrivateChannelManageMembers"] = *c.TeamSettings.RestrictPrivateChannelManageMembers
	props["EnableXToLeaveChannelsFromLHS"] = strconv.FormatBool(*c.TeamSettings.EnableXToLeaveChannelsFromLHS)
	props["TeammateNameDisplay"] = *c.TeamSettings.TeammateNameDisplay
	props["ExperimentalPrimaryTeam"] = *c.TeamSettings.ExperimentalPrimaryTeam

	props["AndroidLatestVersion"] = c.ClientRequirements.AndroidLatestVersion
	props["AndroidMinVersion"] = c.ClientRequirements.AndroidMinVersion
	props["DesktopLatestVersion"] = c.ClientRequirements.DesktopLatestVersion
	props["DesktopMinVersion"] = c.ClientRequirements.DesktopMinVersion
	props["IosLatestVersion"] = c.ClientRequirements.IosLatestVersion
	props["IosMinVersion"] = c.ClientRequirements.IosMinVersion

	props["EnableOAuthServiceProvider"] = strconv.FormatBool(c.ServiceSettings.EnableOAuthServiceProvider)
	props["GoogleDeveloperKey"] = c.ServiceSettings.GoogleDeveloperKey
	props["EnableIncomingWebhooks"] = strconv.FormatBool(c.ServiceSettings.EnableIncomingWebhooks)
	props["EnableOutgoingWebhooks"] = strconv.FormatBool(c.ServiceSettings.EnableOutgoingWebhooks)
	props["EnableCommands"] = strconv.FormatBool(*c.ServiceSettings.EnableCommands)
	props["EnableOnlyAdminIntegrations"] = strconv.FormatBool(*c.ServiceSettings.EnableOnlyAdminIntegrations)
	props["EnablePostUsernameOverride"] = strconv.FormatBool(c.ServiceSettings.EnablePostUsernameOverride)
	props["EnablePostIconOverride"] = strconv.FormatBool(c.ServiceSettings.EnablePostIconOverride)
	props["EnableUserAccessTokens"] = strconv.FormatBool(*c.ServiceSettings.EnableUserAccessTokens)
	props["EnableLinkPreviews"] = strconv.FormatBool(*c.ServiceSettings.EnableLinkPreviews)
	props["EnableTesting"] = strconv.FormatBool(c.ServiceSettings.EnableTesting)
	props["EnableDeveloper"] = strconv.FormatBool(*c.ServiceSettings.EnableDeveloper)
	props["EnableDiagnostics"] = strconv.FormatBool(*c.LogSettings.EnableDiagnostics)
	props["RestrictPostDelete"] = *c.ServiceSettings.RestrictPostDelete
	props["AllowEditPost"] = *c.ServiceSettings.AllowEditPost
	props["PostEditTimeLimit"] = fmt.Sprintf("%v", *c.ServiceSettings.PostEditTimeLimit)
	props["CloseUnusedDirectMessages"] = strconv.FormatBool(*c.ServiceSettings.CloseUnusedDirectMessages)
	props["EnablePreviewFeatures"] = strconv.FormatBool(*c.ServiceSettings.EnablePreviewFeatures)
	props["EnableTutorial"] = strconv.FormatBool(*c.ServiceSettings.EnableTutorial)
	props["ExperimentalEnableDefaultChannelLeaveJoinMessages"] = strconv.FormatBool(*c.ServiceSettings.ExperimentalEnableDefaultChannelLeaveJoinMessages)
	props["ExperimentalGroupUnreadChannels"] = *c.ServiceSettings.ExperimentalGroupUnreadChannels

	props["SendEmailNotifications"] = strconv.FormatBool(c.EmailSettings.SendEmailNotifications)
	props["SendPushNotifications"] = strconv.FormatBool(*c.EmailSettings.SendPushNotifications)
	props["EnableSignUpWithEmail"] = strconv.FormatBool(c.EmailSettings.EnableSignUpWithEmail)
	props["EnableSignInWithEmail"] = strconv.FormatBool(*c.EmailSettings.EnableSignInWithEmail)
	props["EnableSignInWithUsername"] = strconv.FormatBool(*c.EmailSettings.EnableSignInWithUsername)
	props["RequireEmailVerification"] = strconv.FormatBool(c.EmailSettings.RequireEmailVerification)
	props["EnableEmailBatching"] = strconv.FormatBool(*c.EmailSettings.EnableEmailBatching)
	props["EmailNotificationContentsType"] = *c.EmailSettings.EmailNotificationContentsType

	props["EmailLoginButtonColor"] = *c.EmailSettings.LoginButtonColor
	props["EmailLoginButtonBorderColor"] = *c.EmailSettings.LoginButtonBorderColor
	props["EmailLoginButtonTextColor"] = *c.EmailSettings.LoginButtonTextColor

	props["EnableSignUpWithGitLab"] = strconv.FormatBool(c.GitLabSettings.Enable)

	props["ShowEmailAddress"] = strconv.FormatBool(c.PrivacySettings.ShowEmailAddress)

	props["TermsOfServiceLink"] = *c.SupportSettings.TermsOfServiceLink
	props["PrivacyPolicyLink"] = *c.SupportSettings.PrivacyPolicyLink
	props["AboutLink"] = *c.SupportSettings.AboutLink
	props["HelpLink"] = *c.SupportSettings.HelpLink
	props["ReportAProblemLink"] = *c.SupportSettings.ReportAProblemLink
	props["SupportEmail"] = *c.SupportSettings.SupportEmail

	props["EnableFileAttachments"] = strconv.FormatBool(*c.FileSettings.EnableFileAttachments)
	props["EnableMobileFileUpload"] = strconv.FormatBool(*c.FileSettings.EnableMobileUpload)
	props["EnableMobileFileDownload"] = strconv.FormatBool(*c.FileSettings.EnableMobileDownload)
	props["EnablePublicLink"] = strconv.FormatBool(c.FileSettings.EnablePublicLink)

	props["WebsocketPort"] = fmt.Sprintf("%v", *c.ServiceSettings.WebsocketPort)
	props["WebsocketSecurePort"] = fmt.Sprintf("%v", *c.ServiceSettings.WebsocketSecurePort)

	props["DefaultClientLocale"] = *c.LocalizationSettings.DefaultClientLocale
	props["AvailableLocales"] = *c.LocalizationSettings.AvailableLocales
	props["SQLDriverName"] = *c.SqlSettings.DriverName

	props["EnableCustomEmoji"] = strconv.FormatBool(*c.ServiceSettings.EnableCustomEmoji)
	props["EnableEmojiPicker"] = strconv.FormatBool(*c.ServiceSettings.EnableEmojiPicker)
	props["RestrictCustomEmojiCreation"] = *c.ServiceSettings.RestrictCustomEmojiCreation
	props["MaxFileSize"] = strconv.FormatInt(*c.FileSettings.MaxFileSize, 10)
	props["AppDownloadLink"] = *c.NativeAppSettings.AppDownloadLink
	props["AndroidAppDownloadLink"] = *c.NativeAppSettings.AndroidAppDownloadLink
	props["IosAppDownloadLink"] = *c.NativeAppSettings.IosAppDownloadLink

	props["EnableWebrtc"] = strconv.FormatBool(*c.WebrtcSettings.Enable)

	props["MaxNotificationsPerChannel"] = strconv.FormatInt(*c.TeamSettings.MaxNotificationsPerChannel, 10)
	props["EnableConfirmNotificationsToChannel"] = strconv.FormatBool(*c.TeamSettings.EnableConfirmNotificationsToChannel)
	props["TimeBetweenUserTypingUpdatesMilliseconds"] = strconv.FormatInt(*c.ServiceSettings.TimeBetweenUserTypingUpdatesMilliseconds, 10)
	props["EnableUserTypingMessages"] = strconv.FormatBool(*c.ServiceSettings.EnableUserTypingMessages)
	props["EnableChannelViewedMessages"] = strconv.FormatBool(*c.ServiceSettings.EnableChannelViewedMessages)

	props["DiagnosticId"] = diagnosticId
	props["DiagnosticsEnabled"] = strconv.FormatBool(*c.LogSettings.EnableDiagnostics)

	props["PluginsEnabled"] = strconv.FormatBool(*c.PluginSettings.Enable)

	hasImageProxy := c.ServiceSettings.ImageProxyType != nil && *c.ServiceSettings.ImageProxyType != "" && c.ServiceSettings.ImageProxyURL != nil && *c.ServiceSettings.ImageProxyURL != ""
	props["HasImageProxy"] = strconv.FormatBool(hasImageProxy)

	if IsLicensed() {
		License := License()
		props["ExperimentalTownSquareIsReadOnly"] = strconv.FormatBool(*c.TeamSettings.ExperimentalTownSquareIsReadOnly)
		props["ExperimentalEnableAuthenticationTransfer"] = strconv.FormatBool(*c.ServiceSettings.ExperimentalEnableAuthenticationTransfer)

		if *License.Features.CustomBrand {
			props["EnableCustomBrand"] = strconv.FormatBool(*c.TeamSettings.EnableCustomBrand)
			props["CustomBrandText"] = *c.TeamSettings.CustomBrandText
			props["CustomDescriptionText"] = *c.TeamSettings.CustomDescriptionText
		}

		if *License.Features.LDAP {
			props["EnableLdap"] = strconv.FormatBool(*c.LdapSettings.Enable)
			props["LdapLoginFieldName"] = *c.LdapSettings.LoginFieldName
			props["LdapNicknameAttributeSet"] = strconv.FormatBool(*c.LdapSettings.NicknameAttribute != "")
			props["LdapFirstNameAttributeSet"] = strconv.FormatBool(*c.LdapSettings.FirstNameAttribute != "")
			props["LdapLastNameAttributeSet"] = strconv.FormatBool(*c.LdapSettings.LastNameAttribute != "")
			props["LdapLoginButtonColor"] = *c.LdapSettings.LoginButtonColor
			props["LdapLoginButtonBorderColor"] = *c.LdapSettings.LoginButtonBorderColor
			props["LdapLoginButtonTextColor"] = *c.LdapSettings.LoginButtonTextColor
		}

		if *License.Features.MFA {
			props["EnableMultifactorAuthentication"] = strconv.FormatBool(*c.ServiceSettings.EnableMultifactorAuthentication)
			props["EnforceMultifactorAuthentication"] = strconv.FormatBool(*c.ServiceSettings.EnforceMultifactorAuthentication)
		}

		if *License.Features.Compliance {
			props["EnableCompliance"] = strconv.FormatBool(*c.ComplianceSettings.Enable)
		}

		if *License.Features.SAML {
			props["EnableSaml"] = strconv.FormatBool(*c.SamlSettings.Enable)
			props["SamlLoginButtonText"] = *c.SamlSettings.LoginButtonText
			props["SamlFirstNameAttributeSet"] = strconv.FormatBool(*c.SamlSettings.FirstNameAttribute != "")
			props["SamlLastNameAttributeSet"] = strconv.FormatBool(*c.SamlSettings.LastNameAttribute != "")
			props["SamlNicknameAttributeSet"] = strconv.FormatBool(*c.SamlSettings.NicknameAttribute != "")
			props["SamlLoginButtonColor"] = *c.SamlSettings.LoginButtonColor
			props["SamlLoginButtonBorderColor"] = *c.SamlSettings.LoginButtonBorderColor
			props["SamlLoginButtonTextColor"] = *c.SamlSettings.LoginButtonTextColor
		}

		if *License.Features.Cluster {
			props["EnableCluster"] = strconv.FormatBool(*c.ClusterSettings.Enable)
		}

		if *License.Features.Cluster {
			props["EnableMetrics"] = strconv.FormatBool(*c.MetricsSettings.Enable)
		}

		if *License.Features.GoogleOAuth {
			props["EnableSignUpWithGoogle"] = strconv.FormatBool(c.GoogleSettings.Enable)
		}

		if *License.Features.Office365OAuth {
			props["EnableSignUpWithOffice365"] = strconv.FormatBool(c.Office365Settings.Enable)
		}

		if *License.Features.PasswordRequirements {
			props["PasswordMinimumLength"] = fmt.Sprintf("%v", *c.PasswordSettings.MinimumLength)
			props["PasswordRequireLowercase"] = strconv.FormatBool(*c.PasswordSettings.Lowercase)
			props["PasswordRequireUppercase"] = strconv.FormatBool(*c.PasswordSettings.Uppercase)
			props["PasswordRequireNumber"] = strconv.FormatBool(*c.PasswordSettings.Number)
			props["PasswordRequireSymbol"] = strconv.FormatBool(*c.PasswordSettings.Symbol)
		}

		if *License.Features.Announcement {
			props["EnableBanner"] = strconv.FormatBool(*c.AnnouncementSettings.EnableBanner)
			props["BannerText"] = *c.AnnouncementSettings.BannerText
			props["BannerColor"] = *c.AnnouncementSettings.BannerColor
			props["BannerTextColor"] = *c.AnnouncementSettings.BannerTextColor
			props["AllowBannerDismissal"] = strconv.FormatBool(*c.AnnouncementSettings.AllowBannerDismissal)
		}

		if *License.Features.ThemeManagement {
			props["EnableThemeSelection"] = strconv.FormatBool(*c.ThemeSettings.EnableThemeSelection)
			props["DefaultTheme"] = *c.ThemeSettings.DefaultTheme
			props["AllowCustomThemes"] = strconv.FormatBool(*c.ThemeSettings.AllowCustomThemes)
			props["AllowedThemes"] = strings.Join(c.ThemeSettings.AllowedThemes, ",")
		}

		if *License.Features.DataRetention {
			props["DataRetentionEnableMessageDeletion"] = strconv.FormatBool(*c.DataRetentionSettings.EnableMessageDeletion)
			props["DataRetentionMessageRetentionDays"] = strconv.FormatInt(int64(*c.DataRetentionSettings.MessageRetentionDays), 10)
			props["DataRetentionEnableFileDeletion"] = strconv.FormatBool(*c.DataRetentionSettings.EnableFileDeletion)
			props["DataRetentionFileRetentionDays"] = strconv.FormatInt(int64(*c.DataRetentionSettings.FileRetentionDays), 10)
		}
	}

	return props
}

func ValidateLdapFilter(cfg *model.Config, ldap einterfaces.LdapInterface) *model.AppError {
	if *cfg.LdapSettings.Enable && ldap != nil && *cfg.LdapSettings.UserFilter != "" {
		if err := ldap.ValidateFilter(*cfg.LdapSettings.UserFilter); err != nil {
			return err
		}
	}
	return nil
}

func ValidateLocales(cfg *model.Config) *model.AppError {
	var err *model.AppError
	locales := GetSupportedLocales()
	if _, ok := locales[*cfg.LocalizationSettings.DefaultServerLocale]; !ok {
		*cfg.LocalizationSettings.DefaultServerLocale = model.DEFAULT_LOCALE
		err = model.NewAppError("ValidateLocales", "utils.config.supported_server_locale.app_error", nil, "", http.StatusBadRequest)
	}

	if _, ok := locales[*cfg.LocalizationSettings.DefaultClientLocale]; !ok {
		*cfg.LocalizationSettings.DefaultClientLocale = model.DEFAULT_LOCALE
		err = model.NewAppError("ValidateLocales", "utils.config.supported_client_locale.app_error", nil, "", http.StatusBadRequest)
	}

	if len(*cfg.LocalizationSettings.AvailableLocales) > 0 {
		isDefaultClientLocaleInAvailableLocales := false
		for _, word := range strings.Split(*cfg.LocalizationSettings.AvailableLocales, ",") {
			if _, ok := locales[word]; !ok {
				*cfg.LocalizationSettings.AvailableLocales = ""
				isDefaultClientLocaleInAvailableLocales = true
				err = model.NewAppError("ValidateLocales", "utils.config.supported_available_locales.app_error", nil, "", http.StatusBadRequest)
				break
			}

			if word == *cfg.LocalizationSettings.DefaultClientLocale {
				isDefaultClientLocaleInAvailableLocales = true
			}
		}

		availableLocales := *cfg.LocalizationSettings.AvailableLocales

		if !isDefaultClientLocaleInAvailableLocales {
			availableLocales += "," + *cfg.LocalizationSettings.DefaultClientLocale
			err = model.NewAppError("ValidateLocales", "utils.config.add_client_locale.app_error", nil, "", http.StatusBadRequest)
		}

		*cfg.LocalizationSettings.AvailableLocales = strings.Join(RemoveDuplicatesFromStringArray(strings.Split(availableLocales, ",")), ",")
	}

	return err
}