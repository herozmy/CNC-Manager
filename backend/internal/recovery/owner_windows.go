package recovery

import "io/fs"

// Windows files inherit the data directory ACL. The portable server and maintenance run as the same user.
func restoreOwner(string, fs.FileInfo) error { return nil }
