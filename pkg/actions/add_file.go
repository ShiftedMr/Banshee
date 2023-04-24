// Do a find and replace for a string during a migration
package actions

type AddFile struct {
	BaseDir string
	NewFile string `fig:"file"`
	Content string `fig:"content"`
}

func NewAddFileAction(dir string, description string, input map[string]string) *AddFile {
	return &AddFile{
		BaseDir: dir,
		NewFile: input["file"],
		Content: input["content"],
	}
}

func (r *AddFile) Run() error {
	return nil
}
