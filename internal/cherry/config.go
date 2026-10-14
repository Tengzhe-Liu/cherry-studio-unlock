package cherry

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/cherry-tools/cherry-agent-manager/internal/i18n"
)

// configFileName 是工具自己的配置文件，放在 Cherry Studio 数据目录下。
// 只存"上次确认可用的 app.asar 路径"这类信息，不碰 Cherry Studio 自己的任何配置。
const configFileName = "cherry-agent-manager.json"

// Config 是工具的持久化配置。
type Config struct {
	// AsarPath 是上次成功定位到的 app.asar。
	//
	// 为什么要存它：Cherry Studio 必须关闭才能写数据，而关闭后就无法从进程反查安装目录了。
	// 用户如果把程序装在自定义目录（例如 C:\app\AI\Cherry Studio），
	// 仅靠常见安装位置是找不到的——那正好是最需要写数据的场景。
	AsarPath string `json:"asarPath,omitempty"`
	// Lang 是上次选择的界面语言，下次启动沿用。
	Lang string `json:"lang,omitempty"`
}

// ConfigPath 返回配置文件的完整路径。
func ConfigPath(dataDir string) string {
	return filepath.Join(dataDir, configFileName)
}

// LoadConfig 读取配置。文件不存在或损坏时返回空配置，不报错——
// 配置只是加速手段，缺了也应该能继续用。
func LoadConfig(dataDir string) *Config {
	raw, err := os.ReadFile(ConfigPath(dataDir))
	if err != nil {
		return &Config{}
	}
	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return &Config{}
	}
	return &c
}

// SaveConfig 写入配置。
func SaveConfig(dataDir string, c *Config) error {
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	path := ConfigPath(dataDir)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return i18n.Wrap("err.writeConfig", err)
	}
	return os.Rename(tmp, path)
}

// RememberAsarPath 把定位到的 app.asar 路径记下来，供下次（Cherry Studio 关闭时）使用。
func RememberAsarPath(dataDir, asarPath string) error {
	c := LoadConfig(dataDir)
	if c.AsarPath == asarPath {
		return nil
	}
	c.AsarPath = asarPath
	return SaveConfig(dataDir, c)
}
