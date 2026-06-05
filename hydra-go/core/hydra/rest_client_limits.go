package hydra

// RESTClientLimits holds optional client-go REST rate limits (QPS/burst) for Kubernetes API calls
// tied to this cluster instance. The zero value leaves client-go defaults (typically 5 QPS / 10 burst).
type RESTClientLimits struct {
	QPS   float32
	Burst int
}
