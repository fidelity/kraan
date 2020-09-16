package repos

var (
	NewRepo        = newRepo
	FetchArtifact  = fetchArtifact
	SetTarConsumer = Repo.setTarConsumer
	GetGitRepo     = Repo.getGitRepo
	GetDataPath    = getDataPath
)
