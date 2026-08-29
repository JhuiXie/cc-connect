package core

func AgentFileSendPromptForLang(lang Language) string {
	switch lang {
	case LangChinese, LangTraditionalChinese:
		return `### 将本地文件发送给用户
当用户明确要求把 WSL 中的文件或你生成的文件发给他时，使用 Bash 运行：

  cc-connect send --file "/absolute/path/to/file"

图片使用 'cc-connect send --image "/absolute/path/to/image.png"'。可按需附加 '--message "说明"'，也可以重复 '--file' 或 '--image'。
CC_PROJECT、CC_SESSION_KEY 和 CC_DATA_DIR 已由 cc-connect 自动设置，不要猜测或手动替换它们。
不要为了发送文件而读取或内嵌文件内容；命令成功退出后才能告诉用户已发送，失败时必须报告错误。
除非用户明确要求，不要发送密钥、凭据、'.env' 或其他敏感文件。`
	default:
		return `### Send a local file to the user
When the user explicitly asks you to send a WSL file or an artifact you generated, use Bash:

  cc-connect send --file "/absolute/path/to/file"

Use 'cc-connect send --image "/absolute/path/to/image.png"' for images. Add '--message "note"' when useful, and repeat '--file' or '--image' when needed.
CC_PROJECT, CC_SESSION_KEY, and CC_DATA_DIR are set automatically by cc-connect; do not guess or replace them.
Do not read or inline file contents just to send a file. Claim success only after the command exits successfully; report the error otherwise.
Unless explicitly requested, do not send secrets, credentials, '.env', or other sensitive files.`
	}
}
