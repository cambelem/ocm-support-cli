package sync_cloud_resources

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "sync-cloud-resources [branch-name] [csv-path]",
	Short: "Syncs cloud resources in AMS and generates quota rules for them",
	Long:  "Syncs cloud resources in AMS and generates quota rules for them",
	RunE:  syncCloudResources,
	Args:  cobra.ExactArgs(2),
}

var args struct {
	dryRun bool
}

func init() {
	flags := Cmd.Flags()
	flags.BoolVar(&args.dryRun, "dry-run", true, "If false, it commits the generated cloud resources and quota rule changes to the remote branch at https://gitlab.cee.redhat.com/service/uhc-account-manager")
}

func syncCloudResources(cmd *cobra.Command, argv []string) error {
	branchName := argv[0]
	csvPath := argv[1]
	if branchName == "" {
		return fmt.Errorf("branch name cannot be empty")
	}
	if csvPath == "" {
		return fmt.Errorf("csv path cannot be empty")
	}

	fmt.Println("Validating branch name", branchName)
	err := CheckRefFormat(branchName)
	if err != nil {
		return fmt.Errorf("incorrect branch name: %v", err)
	}

	fmt.Println("Validating csv path", csvPath)
	err = CheckIfFileExists(csvPath)
	if err != nil {
		return fmt.Errorf("an error occurred while checking if the csv files exists: %v", err)
	}

	fmt.Println("Creating temporary directory")
	tempDir, err := os.MkdirTemp(os.TempDir(), branchName)
	if err != nil {
		return fmt.Errorf("an error occurred while creating temporary directory: %v", err)
	}

	amsUpstreamRepo := "git@gitlab.cee.redhat.com:service/uhc-account-manager.git"
	amsRepo := gitRepo{
		repoUrl:   amsUpstreamRepo,
		localPath: tempDir,
	}

	fmt.Println("Cloning AMS repo at:", tempDir)
	err = amsRepo.Clone()
	if err != nil {
		return fmt.Errorf("an error occurred while cloning AMS repo: %v", err)
	}

	fmt.Println("Creating a new branch")
	err = amsRepo.Branch(branchName)
	if err != nil {
		return fmt.Errorf("an error occurred while creating a new branch: %v", err)
	}

	fmt.Println("Replacing cloud resources file")
	err = ReplaceFileContent(fmt.Sprintf("%s/config/quota-cloud-resources.csv", tempDir), csvPath)
	if err != nil {
		return fmt.Errorf("an error occurred while getting the head of the reference: %v", err)
	}

	fmt.Println("Generating quota rules")
	_, err = ExecuteCmd(fmt.Sprintf("cd %s && make install && make binary", tempDir))
	if err != nil {
		return fmt.Errorf("an error occurred while building AMS: %v", err)
	}
	// Run account-manager command with an absolute path so that valid binary is found and executed
	// cd is done here as GetProjectRootDir() in AMS detects the directory based on the presence of .git
	// Ref: https://gitlab.cee.redhat.com/service/uhc-account-manager/-/blob/master/pkg/config/config.go?ref_type=heads#L215
	_, err = ExecuteCmd(fmt.Sprintf("cd %s && %s/account-manager generate quota", tempDir, tempDir))
	if err != nil {
		return fmt.Errorf("an error occurred while generating quota rules: %v", err)
	}

	fmt.Println("Staging changes")
	err = amsRepo.StageAllFiles()
	if err != nil {
		return fmt.Errorf("an error occurred while staging the files: %v", err)
	}

	fmt.Println("Committing changes")
	err = amsRepo.Commit(fmt.Sprintf("Syncing cloud resources and quota rules for %s", branchName))
	if err != nil {
		return fmt.Errorf("an error occurred committing the changes: %v", err)
	}

	fmt.Println("Pushing changes to remote branch")
	if args.dryRun {
		fmt.Println("DRY RUN: Would push the changes to remote branch:", branchName)
		return nil
	}
	err = amsRepo.Push("origin", branchName)
	if err != nil {
		return fmt.Errorf("an error occurred while pushing the changes: %v", err)
	}
	return nil
}

func ReplaceFileContent(originalFilePath, filePathWithUpdatedText string) error {
	originalFile, err := os.OpenFile(originalFilePath, os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer originalFile.Close()

	newFile, err := os.Open(filePathWithUpdatedText)
	if err != nil {
		return err
	}
	defer newFile.Close()
	_, err = io.Copy(originalFile, newFile)
	if err != nil {
		return err
	}
	err = originalFile.Sync()
	return err
}

func CheckIfFileExists(filepath string) error {
	if _, err := os.Stat(filepath); errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

type gitRepo struct {
	repoUrl   string
	localPath string
}

func ExecuteCmd(command string) (string, error) {
	fmt.Println(command)
	app := "bash"
	arg1 := "-c"
	cmd := exec.Command(app, arg1, command)
	var stdBuffer bytes.Buffer
	mw := io.MultiWriter(os.Stdout, &stdBuffer)
	cmd.Stdout = mw
	cmd.Stderr = mw
	err := cmd.Run()
	if err != nil {
		err := fmt.Errorf("%v : %s", err, stdBuffer.String())
		return "", err
	}
	return stdBuffer.String(), nil
}

func CheckRefFormat(branchName string) error {
	_, err := ExecuteCmd(fmt.Sprintf("git check-ref-format --branch %s", branchName))
	if err != nil {
		return err
	}
	return nil
}

func (g gitRepo) Clone() error {
	_, err := ExecuteCmd(fmt.Sprintf("git clone %s %s", g.repoUrl, g.localPath))
	return err
}

func (g gitRepo) RemoteAdd(remoteName, remoteUrl string) error {
	_, err := ExecuteCmd(fmt.Sprintf("git -C %s remote add %s %s", g.localPath, remoteName, remoteUrl))
	return err
}

func (g gitRepo) Branch(branchName string) error {
	_, err := ExecuteCmd(fmt.Sprintf("git -C %s checkout -b %s", g.localPath, branchName))
	return err
}

func (g gitRepo) StageFiles(files *[]string) error {
	for _, file := range *files {
		_, err := ExecuteCmd(fmt.Sprintf("git -C %s stage %s", g.localPath, file))
		if err != nil {
			return err
		}
	}
	return nil
}

func (g gitRepo) StageAllFiles() error {
	_, err := ExecuteCmd(fmt.Sprintf("git -C %s stage .", g.localPath))
	return err
}

func (g gitRepo) Commit(message string) error {
	_, err := ExecuteCmd(fmt.Sprintf("git -C %s commit -m \"%s\"", g.localPath, message))
	return err
}

func (g gitRepo) Push(remote, remoteUrl string) error {
	_, err := ExecuteCmd(fmt.Sprintf("git -C %s push %s %s", g.localPath, remote, remoteUrl))
	return err
}
