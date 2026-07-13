package ids

type Generator interface{ NewID(kind string) string }
