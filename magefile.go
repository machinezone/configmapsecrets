//+build mage

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/machinezone/configmapsecrets/pkg/genapi"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	magetarget "github.com/magefile/mage/target"
)

const (
	name = "configmapsecret-controller"
	repo = "github.com/machinezone/configmapsecrets"

	goVersion  = "1.15"
	buildImage = "golang:" + goVersion + "-alpine"
	testImage  = "kubebuilder-golang-" + goVersion + "-alpine"
	baseImage  = "gcr.io/distroless/static:latest"
)

var arches = []string{"amd64", "arm", "arm64"}

var trg = target{name: name, repo: repo}

type target struct {
	name     string
	repo     string
	version  string
	revision string
	branch   string
}

func (t *target) Name() string { return t.name }

func (t *target) Repo() string { return t.repo }

func (t *target) Version() string {
	mg.Deps(t.initRepoData)
	return t.version
}

func (t *target) Revision() string {
	mg.Deps(t.initRepoData)
	return t.revision
}

func (t *target) Branch() string {
	mg.Deps(t.initRepoData)
	return t.branch
}

func (t *target) Registry() string {
	if s := os.Getenv("REGISTRY"); s != "" {
		return s
	}
	return "registry.hub.docker.com/mzinc"
}

func (t *target) initRepoData() error {
	var err error
	t.version, err = sh.Output("git", "describe", "--tags", "--always", "--dirty", "--abbrev=12")
	if err != nil {
		return err
	}
	t.revision, err = sh.Output("git", "rev-parse", "--verify", "HEAD")
	if err != nil {
		return err
	}
	t.branch, err = sh.Output("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	return nil
}

// Prints version.
func Version() error {
	fmt.Println(trg.Version())
	return nil
}

// Launches an interactive shell in a containerized test environment.
func Shell() error {
	mg.Deps(buildTestImg, mkBuildDirs)

	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	args, err := withUserArgs(
		"run",
		"-ti",
		"--rm",
	)
	if err != nil {
		return err
	}
	args = append(args,
		"-w", "/src",
		"-v", pwd+":/src",
		"-v", cachePath("bin")+":/go/bin",
		"-v", cachePath("cache")+":/go/cache",
		"--env", "CGO_ENABLED=0",
		"--env", "GO111MODULE=on",
		"--env", "GOCACHE=/go/cache",
		"--env", "HTTP_PROXY="+os.Getenv("HTTP_PROXY"),
		"--env", "HTTPS_PROXY="+os.Getenv("HTTPS_PROXY"),
		testImage,
		"/bin/sh",
	)
	cmd := exec.Command("docker", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Runs tests.
func Test() error {
	testPath := cachePath("test")
	if ok, err := shouldDo(testPath); !ok {
		return err
	}
	mg.Deps(buildTestImg, mkBuildDirs)

	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	args, err := withUserArgs(
		"run",
		"-i",
		"--rm",
	)
	if err != nil {
		return err
	}
	args = append(args,
		"-w", "/src",
		"-v", pwd+":/src",
		"-v", cachePath("bin")+":/go/bin",
		"-v", cachePath("cache")+":/go/cache",
		"--env", "CGO_ENABLED=0",
		"--env", "GO111MODULE=on",
		"--env", "GOCACHE=/go/cache",
		"--env", "HTTP_PROXY="+os.Getenv("HTTP_PROXY"),
		"--env", "HTTPS_PROXY="+os.Getenv("HTTPS_PROXY"),
		testImage,
		"/bin/sh", "-c", "/src/hack/test.sh cmd pkg",
	)
	if err := sh.RunV("docker", args...); err != nil {
		return err
	}
	return touchFile(testPath)
}

func mkBuildDirs() error {
	if err := mkDir(cachePath("bin")); err != nil {
		return err
	}
	if err := mkDir(cachePath("cache")); err != nil {
		return err
	}
	return nil
}

func buildTestImg() error {
	imgPath := imageBuildPath(testImage)
	if ok, err := fileExists(imgPath); ok || err != nil {
		return err
	}
	fmt.Printf("building kubebuilder test image\n")
	mg.Deps(pullBuildImage)

	dir, err := ioutil.TempDir("", "kubebuilder")
	if err != nil {
		return err
	}
	defer rmDir(dir)
	f, err := os.Create(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		return err
	}
	defer f.Close()
	buf := bufio.NewWriter(f)
	fmt.Fprintf(buf, "FROM %s\n", buildImage)
	fmt.Fprintf(buf, `RUN wget -q -O - "https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)"`)
	fmt.Fprintf(buf, " | tar -xz -C /tmp/")
	fmt.Fprintf(buf, " && mv /tmp/kubebuilder* /usr/local/kubebuilder\n")
	if err := buf.Flush(); err != nil {
		return err
	}

	if err := sh.Run("docker", "build", "-t", testImage, dir); err != nil {
		return err
	}
	id, err := sh.Output("docker", "images", "-q", testImage)
	if err != nil {
		return err
	}
	return appendFile(imgPath, id)
}

// Builds target binaries.
func Bins() error {
	if ok, err := shouldDoBins(); !ok {
		return err
	}
	fmt.Printf("building %s binaries in %s\n", trg.Name(), buildImage)
	mg.Deps(pullBuildImage, mkBuildDirs)

	now := time.Now()
	cmds := []string{
		fmt.Sprintf("export LDFLAGS=%q", buildinfoLDFlags(
			"binary", trg.Name(),
			"version", trg.Version(),
			"repo", trg.Repo(),
			"branch", trg.Branch(),
			"revision", trg.Revision(),
			"buildUnix", strconv.FormatInt(now.Unix(), 10),
		)),
	}
	src := "/src/cmd/" + trg.Name()
	for _, arch := range arches {
		if err := mkDir(binDir(arch)); err != nil {
			return err
		}
		cmds = append(cmds,
			fmt.Sprintf(`echo "building binary for linux/%s"`, arch),
			fmt.Sprintf("export GOARCH=%s", arch),
			fmt.Sprintf(
				`go build -mod vendor -ldflags "$${LDFLAGS}" -o %q %q`,
				"/go/bin/linux_$${GOARCH}/"+trg.Name(), src,
			),
		)
	}
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	args, err := withUserArgs(
		"run",
		"-i",
		"--rm",
	)
	if err != nil {
		return err
	}
	args = append(args,
		"-w", "/src",
		"-v", pwd+":/src",
		"-v", cachePath("bin")+":/go/bin",
		"-v", cachePath("cache")+":/go/cache",
		"--env", "CGO_ENABLED=0",
		"--env", "GO111MODULE=on",
		"--env", "GOCACHE=/go/cache",
		"--env", "GOOS=linux",
		"--env", "HTTP_PROXY="+os.Getenv("HTTP_PROXY"),
		"--env", "HTTPS_PROXY="+os.Getenv("HTTPS_PROXY"),
		buildImage,
		"/bin/sh", "-c", strings.Join(cmds, " && "),
	)
	env := map[string]string{"$": "$"} // Escape hack: "$$LDFLAGS" becomes "$LDFLAGS"
	if _, err := sh.Exec(env, os.Stdout, os.Stderr, "docker", args...); err != nil {
		return err
	}
	for _, arch := range arches {
		if err := os.Chtimes(binPath(arch), now, now); err != nil {
			return err
		}
	}
	return nil
}

func buildinfoLDFlags(namesAndValues ...string) string {
	var flags []string
	for i := 0; i < len(namesAndValues); i += 2 {
		key := namesAndValues[i]
		val := namesAndValues[i+1]
		flags = append(flags, fmt.Sprintf("-X %s/pkg/buildinfo.%s=%v", trg.Repo(), key, val))
	}
	return strings.Join(flags, " ")
}

// Builds container images.
func Imgs() error {
	if ok, err := shouldDoImgs(); !ok {
		return err
	}
	mg.Deps(Bins, pullBaseImage)
	fmt.Printf("building %s images from %s\n", manifest(), baseImage)

	for _, arch := range arches {
		if err := buildImg(arch); err != nil {
			return err
		}
	}
	return nil
}

func buildImg(arch string) error {
	fmt.Printf("building image for linux/%s\n", arch)

	// Write temporary dockerfile
	tmp, err := ioutil.TempFile("", arch)
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	buf := bufio.NewWriter(tmp)
	fmt.Fprintf(buf, "FROM %s\n", baseImage)
	fmt.Fprintf(buf, "ADD LICENSE /LICENSE\n")
	fmt.Fprintf(buf, "LABEL os=linux")
	fmt.Fprintf(buf, " arch=%s", arch)
	fmt.Fprintf(buf, " binary=%s", trg.Name())
	fmt.Fprintf(buf, " repository=%s", trg.Repo())
	fmt.Fprintf(buf, " version=%s", trg.Version())
	fmt.Fprintf(buf, " revision=%s", trg.Revision())
	fmt.Fprintf(buf, " branch=%s", trg.Branch())
	fmt.Fprintf(buf, "\n")
	fmt.Fprintf(buf, "ADD %s /%s\n", trg.Name(), trg.Name())
	fmt.Fprintf(buf, "USER 65535:65535\n") // distroless doesn't have "nobody"
	fmt.Fprintf(buf, "ENTRYPOINT [%q]\n", "/"+trg.Name())
	if err := buf.Flush(); err != nil {
		return err
	}

	ctxDir := binDir(arch)
	curDir, err := os.Getwd()
	if err != nil {
		return err
	}
	srcLicense := path.Join(curDir, "LICENSE")
	dstLicense := path.Join(ctxDir, "LICENSE")
	if err := os.Link(srcLicense, dstLicense); err != nil && !os.IsExist(err) {
		return err
	}

	tag := image(arch)
	err = sh.Run(
		"docker",
		"build",
		"--platform", "linux/"+arch,
		"-t", tag,
		"-f", tmp.Name(), // dockerfile
		ctxDir, // context: just the binary
	)
	if err != nil {
		return err
	}
	id, err := sh.Output("docker", "images", "-q", tag)
	if err != nil {
		return err
	}
	return appendFile(imageBuildPath(tag), id)
}

// Pushes container images.
func Push() error {
	if ok, err := shouldDoPush(); !ok {
		return err
	}
	mg.Deps(Imgs)

	base := manifest()
	fmt.Printf("pushing %s images\n", base)

	// push images
	var tags []string
	for _, arch := range arches {
		fmt.Printf("pushing image for linux/%s\n", arch)
		src := image(arch)
		tag := archTag(arch)
		if err := sh.Run("docker", "tag", src, tag); err != nil {
			return err
		}
		if err := sh.Run("docker", "push", tag); err != nil {
			return err
		}
		digest, err := sh.Output("docker", "inspect", "--format={{index .RepoDigests 0}}", tag)
		if err != nil {
			return err
		}
		tags = append(tags, digest)
	}

	// create and push manifest
	fmt.Printf("pushing manifest\n")
	env := map[string]string{"DOCKER_CLI_EXPERIMENTAL": "enabled"}
	args := append([]string{"manifest", "create", "--amend", base}, tags...)
	if out, err := sh.OutputWith(env, "docker", args...); err != nil {
		fmt.Println(out)
		return err
	}
	for i, arch := range arches {
		err := sh.RunWith(
			env,
			"docker",
			"manifest",
			"annotate",
			base,
			tags[i],
			"--os", "linux",
			"--arch", arch,
		)
		if err != nil {
			return err
		}
	}
	if err := sh.RunWith(env, "docker", "manifest", "push", "--purge", base); err != nil {
		return err
	}
	out, err := sh.OutputWith(env, "docker", "manifest", "inspect", base)
	if err != nil {
		return err
	}
	return writeFile(imagePushPath(base), out)
}

func Generate() error {
	mg.Deps(generateCode, generateCDRs, generateRBAC, generateDocs)

	return nil
}

func generateCode() error {
	return sh.Run("controller-gen", "object:headerFile=./hack/boilerplate.go.txt", "paths=./pkg/api/...")
}

func generateCDRs() error {
	out, err := sh.Output("controller-gen", "crd:crdVersions=v1", "paths=./pkg/...", "output:stdout")
	if err != nil {
		return err
	}
	return writeFile("manifest/customresourcedefinition.yaml", out)
}

func generateRBAC() error {
	out, err := sh.Output("controller-gen", "rbac:roleName=configmapsecret-controller", "paths=./cmd/...;./pkg/...", "output:stdout")
	if err != nil {
		return err
	}
	return writeFile("manifest/roles.yaml", out)
}

func generateDocs() error {
	pkg, err := genapi.ParsePackage("github.com/machinezone/configmapsecrets/pkg/api/v1alpha1")
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(nil)
	if err := genapi.WriteMarkdown(buf, pkg); err != nil {
		return err
	}
	return writeFile("docs/api.md", buf.String())
}

// Removes build artifacts.
func Clean() error {
	ids, err := imageIDs()
	if err != nil {
		return err
	}
	if len(ids) > 0 {
		args := append([]string{"rmi", "-f"}, ids...)
		if err := sh.Run("docker", args...); err != nil {
			return err
		}
	}
	return rmDir(cachePath())
}

func imageIDs() ([]string, error) {
	dir := imageBuildPath("")
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	set := make(map[string]bool)
	for _, info := range infos {
		if info.IsDir() {
			continue
		}
		buf, err := ioutil.ReadFile(filepath.Join(dir, info.Name()))
		if err != nil {
			return nil, err
		}
		for _, line := range bytes.Split(buf, []byte{'\n'}) {
			if id := bytes.TrimSpace(line); len(id) > 0 {
				set[string(id)] = true
			}
		}
	}
	var ids []string
	for id := range set {
		ids = append(ids, id)
	}
	return ids, nil
}

// Prints image.
func Image() error {
	fmt.Println(manifest())
	return nil
}

func manifest() string {
	return fmt.Sprintf("%s/%s:%s", trg.Registry(), trg.Name(), trg.Version())
}

func archTag(arch string) string {
	return fmt.Sprintf("%s/%s:__unstable__linux_%s", trg.Registry(), trg.Name(), arch)
}

func image(arch string) string {
	return manifest() + "__linux_" + arch
}

func pullBuildImage() error { return pullImage(buildImage) }

func pullBaseImage() error { return pullImage(baseImage) }

func pullImage(image string) error {
	path := imagePullPath(image)
	if ok, err := fileExists(path); ok || err != nil {
		return err
	}
	fmt.Printf("pulling %s\n", image)
	if err := sh.Run("docker", "pull", image); err != nil {
		return err
	}
	return touchFile(path)
}

func cachePath(parts ...string) string {
	s, err := filepath.Abs(filepath.Join(append([]string{".mage"}, parts...)...))
	if err != nil {
		mg.Fatal(1, err)
	}
	return s
}

func binDir(arch string) string  { return cachePath("bin", "linux_"+arch) }
func binPath(arch string) string { return filepath.Join(binDir(arch), trg.Name()) }

func imagePath(action, image string) string {
	name := strings.NewReplacer("/", "_", ":", "-", "@", "-").Replace(image)
	return cachePath("img", action, name)
}

func imagePullPath(image string) string  { return imagePath("pull", image) }
func imageBuildPath(image string) string { return imagePath("build", image) }
func imagePushPath(image string) string  { return imagePath("push", image) }

func shouldDoBins() (bool, error) {
	var dsts []string
	for _, arch := range arches {
		dsts = append(dsts, binPath(arch))
	}
	return shouldDo(dsts...)
}

func shouldDoImgs() (bool, error) {
	var dsts []string
	for _, arch := range arches {
		dsts = append(dsts, imageBuildPath(arch))
	}
	return shouldDo(dsts...)
}

func shouldDoPush() (bool, error) {
	return shouldDo(imagePushPath(manifest()))
}

func shouldDo(dsts ...string) (bool, error) {
	dst, _, err := earliestMod(dsts...)
	if err != nil {
		return false, err
	}
	var srcs []string
	for _, glob := range []string{
		"*.go",
		"go.mod",
		"go.sum",
		"cmd",
		"pkg",
		"third_party",
	} {
		matches, err := filepath.Glob(glob)
		if err != nil {
			return false, err
		}
		srcs = append(srcs, matches...)
	}
	return magetarget.Dir(dst, srcs...)
}

func earliestMod(files ...string) (string, time.Time, error) {
	var (
		name string
		mod  time.Time
	)
	for _, file := range files {
		s, err := os.Stat(file)
		if os.IsNotExist(err) {
			return file, time.Time{}, nil
		}
		if err != nil {
			return "", time.Time{}, err
		}
		if t := s.ModTime(); mod.IsZero() || t.Before(mod) {
			name = file
			mod = t
		}
	}
	return name, mod, nil
}

func fileExists(path string) (bool, error) {
	switch _, err := os.Stat(path); {
	case os.IsNotExist(err):
		return false, nil
	case err != nil:
		return false, err
	default:
		return true, nil
	}
}

func touchFile(path string) error {
	log.Printf("touching file: %s", path)
	now := time.Now()
	if err := os.Chtimes(path, now, now); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := mkDir(filepath.Dir(path)); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	return f.Close()
}

func writeFile(path, msg string) error {
	if err := mkDir(filepath.Dir(path)); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return writeLine(f, msg)
}

func appendFile(path, msg string) error {
	if err := mkDir(filepath.Dir(path)); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return writeLine(f, msg)
}

func writeLine(w io.Writer, msg string) error {
	if strings.HasSuffix(msg, "\n") {
		_, err := fmt.Fprint(w, msg)
		return err
	}
	_, err := fmt.Fprintln(w, msg)
	return err
}

func mkDir(path string) error {
	log.Printf("making directory: %s", path)
	return os.MkdirAll(path, 0700)
}

func rmDir(path string) error {
	log.Printf("removing directory: %s", path)
	return os.RemoveAll(path)
}

func withUserArgs(args ...string) ([]string, error) {
	u, err := user.Current()
	if err != nil {
		return nil, err
	}
	if !isPodman() {
		return append(args, "-u", u.Uid+":"+u.Gid), nil
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return nil, err
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return nil, err
	}
	const maxIDs = 1 << 16
	return append(args,
		"-u", fmt.Sprintf("%d:%d", uid, gid),
		"--uidmap", fmt.Sprintf("%d:0:1", uid),
		"--uidmap", fmt.Sprintf("0:1:%d", uid),
		"--uidmap", fmt.Sprintf("%d:%d:%d", uid+1, uid+1, maxIDs-uid),
		"--gidmap", fmt.Sprintf("%d:0:1", gid),
		"--gidmap", fmt.Sprintf("0:1:%d", gid),
		"--gidmap", fmt.Sprintf("%d:%d:%d", gid+1, gid+1, maxIDs-gid),
	), nil
}

var podman struct {
	init sync.Once
	v    bool
}

func isPodman() bool {
	podman.init.Do(func() {
		out, err := sh.Output("docker", "--help")
		if err != nil {
			return
		}
		podman.v = strings.Contains(out, "podman")
	})
	return podman.v
}
