//go:generate mockgen -destination=../mocks/kubectl/mockKubectl.go -package=mocks . Kubectl,Command
//go:generate mockgen -destination=../mocks/kubectl/mockExecProvider.go -package=mocks -source=execProvider.go . ExecProvider
//go:generate mockgen -destination=../mocks/logr/mockLogger.go -package=mocks github.com/go-logr/logr Logger
package kubectl

var (
	KubectlCmd        = kubectlCmd
	NewCommandFactory = newCommandFactory
	GetFactoryPath    = CommandFactory.getPath
	GetLogger         = CommandFactory.getLogger
	GetExecProvider   = CommandFactory.getExecProvider
	GetCommandPath    = Command.getPath
	GetSubCmd         = Command.getSubCmd
	GetArgs           = Command.getArgs
	IsJSONOutput      = Command.isJSONOutput
	AsCommandString   = Command.asString
)

func SetKubectlCmd(command string) {
	kubectlCmd = command
}
