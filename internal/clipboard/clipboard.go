// Package clipboard copies text to the system clipboard via per-platform
// command-line tools.
package clipboard

// Copy writes data to the system clipboard. Implementations are in
// clipboard_<os>.go.
func Copy(data string) error {
	return copyImpl(data)
}
