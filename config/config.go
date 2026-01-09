package config

type Config struct {
	// Директория, в которую будут загружаться репозитории для дальнейшей работы с ними
	BufferDirectory string
	// Репозитории со сценариями автоматизации
	Repositories []Repository
}

type Repository struct {
	// Владелец репозитория
	Owner string
	// Имя репозитория
	RepoName string
	// Сценарии автоматизации обработки веток
	BranchPipelines []BranchPipeline
}

type BranchPipeline struct {
	// Шаблон соответствия ветки
	Template string
	// Исполняемый файл Docker сборки
	DockerFilePath string
	// Команды для удаленного сервера
	RemoteCommands []string
}
