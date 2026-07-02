package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/cneill/smoke/pkg/files"
	"github.com/cneill/smoke/pkg/utils"
)

const metadataVersion = 1

// ProjectBucket is the directory where a set of plans belonging to a specific project repository path live.
type ProjectBucket struct {
	ProjectPath string
	PlansDir    string
}

func NewProjectBucket(plansDir, projectPath string) (ProjectBucket, error) {
	if !filepath.IsAbs(projectPath) {
		return ProjectBucket{}, fmt.Errorf("non-absolute project path %q", projectPath)
	}

	projectPath = filepath.Clean(projectPath)

	p := ProjectBucket{
		ProjectPath: projectPath,
		PlansDir:    plansDir,
	}

	return p, nil
}

func (p ProjectBucket) Slug() string {
	return cleanName(filepath.Base(p.ProjectPath))
}

func (p ProjectBucket) Hash() string {
	sum := sha256.Sum256([]byte(p.ProjectPath))
	return hex.EncodeToString(sum[:])[:12]
}

func (p ProjectBucket) Name() string {
	return p.Slug() + "-" + p.Hash()
}

func (p ProjectBucket) Path() string {
	return filepath.Join(p.PlansDir, p.Name())
}

func (p ProjectBucket) PlanMetadataPath(planID string) string {
	return filepath.Join(p.Path(), planID+".meta.json")
}

func (p ProjectBucket) PlanLogPath(planID string) string {
	return filepath.Join(p.Path(), planID+".jsonl")
}

type Store struct {
	Bucket ProjectBucket
}

type Metadata struct {
	metadataPath string `json:"-"`

	Version     int       `json:"version"`
	ProjectPath string    `json:"project_path"`
	ProjectSlug string    `json:"project_slug"`
	ProjectHash string    `json:"project_hash"`
	BucketName  string    `json:"bucket_name"`
	BucketPath  string    `json:"bucket_path"`
	SessionName string    `json:"session_name"`
	PlanID      string    `json:"plan_id"`
	LogPath     string    `json:"log_path"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsedAt  time.Time `json:"last_used_at"`
}

func (m Metadata) FilePath() string {
	return filepath.Join(m.LogPath, m.PlanID+".meta.json")
}

func NewStore(projectPath string) (*Store, error) {
	plansDir, err := files.PlansDirPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get plans directory: %w", err)
	}

	bucket, err := NewProjectBucket(plansDir, projectPath)
	if err != nil {
		return nil, fmt.Errorf("project plan directory error: %w", err)
	}

	return &Store{
		Bucket: bucket,
	}, nil
}

func (s *Store) NewLazyManager(sessionName string) (*Manager, Metadata, error) {
	planID := newPlanID(sessionName)
	metadata := s.newMetadata(sessionName, planID)

	manager, err := LazyManagerFromMetadata(metadata)
	if err != nil {
		return nil, Metadata{}, err
	}

	return manager, metadata, nil
}

func (s *Store) Open(planID string) (*Manager, Metadata, error) {
	metadata, err := s.ReadMetadata(planID)
	if err != nil {
		return nil, Metadata{}, err
	}

	if metadata.ProjectPath != s.Bucket.ProjectPath {
		return nil, Metadata{}, fmt.Errorf("plan %q belongs to different project %q", planID, metadata.ProjectPath)
	}

	manager, err := ManagerFromMetadata(metadata)
	if err != nil {
		return nil, Metadata{}, err
	}

	metadata.LastUsedAt = time.Now().UTC()
	if err := s.WriteMetadata(metadata); err != nil {
		return nil, Metadata{}, err
	}

	return manager, metadata, nil
}

func (s *Store) List() ([]Metadata, error) {
	entries, err := os.ReadDir(s.Bucket.Path())
	if err != nil {
		if os.IsNotExist(err) {
			return []Metadata{}, nil
		}

		return nil, fmt.Errorf("failed to list plan bucket %q: %w", s.Bucket.Path(), err)
	}

	plans := []Metadata{}

	for _, entry := range entries {
		metadata, ok, err := s.metadataFromEntry(entry)
		if err != nil {
			return nil, err
		}

		if ok {
			plans = append(plans, metadata)
		}
	}

	slices.SortFunc(plans, func(a, b Metadata) int {
		return b.SortTime().Compare(a.SortTime())
	})

	return plans, nil
}

func (s *Store) ReadMetadata(planID string) (Metadata, error) {
	path := s.Bucket.PlanMetadataPath(planID)

	data, err := os.ReadFile(path)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to read plan metadata %q: %w", path, err)
	}

	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return Metadata{}, fmt.Errorf("failed to parse plan metadata %q: %w", path, err)
	}

	return metadata, nil
}

func (s *Store) WriteMetadata(metadata Metadata) error {
	if err := os.MkdirAll(s.Bucket.Path(), 0o755); err != nil {
		return fmt.Errorf("failed to create plan bucket %q: %w", s.Bucket.Path(), err)
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal plan metadata: %w", err)
	}

	data = append(data, '\n')
	path := s.Bucket.PlanMetadataPath(metadata.PlanID)

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write plan metadata %q: %w", path, err)
	}

	return nil
}

func (s *Store) metadataFromEntry(entry os.DirEntry) (Metadata, bool, error) {
	if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".meta.json") {
		return Metadata{}, false, nil
	}

	data, err := os.ReadFile(filepath.Join(s.Bucket.Path(), entry.Name()))
	if err != nil {
		return Metadata{}, false, fmt.Errorf("failed to read plan metadata %q: %w", entry.Name(), err)
	}

	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return Metadata{}, false, fmt.Errorf("failed to parse plan metadata %q: %w", entry.Name(), err)
	}

	if metadata.ProjectPath != s.Bucket.ProjectPath {
		return Metadata{}, false, nil
	}

	if stat, err := os.Stat(metadata.LogPath); err == nil && metadata.LastUsedAt.IsZero() {
		metadata.LastUsedAt = stat.ModTime()
	}

	return metadata, true, nil
}

func (m Metadata) SortTime() time.Time {
	if !m.LastUsedAt.IsZero() {
		return m.LastUsedAt
	}

	return m.CreatedAt
}

func (m Metadata) DisplayName() string {
	return fmt.Sprintf("%s (%s, %s)", m.PlanID, m.SessionName, m.SortTime().UTC().Format(time.DateTime))
}

func (s *Store) newMetadata(sessionName, planID string) Metadata {
	now := time.Now().UTC()

	return Metadata{
		metadataPath: s.Bucket.PlanMetadataPath(planID),
		Version:      metadataVersion,
		ProjectPath:  s.Bucket.ProjectPath,
		ProjectSlug:  s.Bucket.Slug(),
		ProjectHash:  s.Bucket.Hash(),
		BucketName:   s.Bucket.Name(),
		BucketPath:   s.Bucket.Path(),
		SessionName:  sessionName,
		PlanID:       planID,
		LogPath:      s.Bucket.PlanLogPath(planID),
		CreatedAt:    now,
		LastUsedAt:   now,
	}
}

func cleanName(input string) string {
	var sb strings.Builder

	for _, r := range strings.ToLower(input) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}

	trimmed := strings.TrimRight(sb.String(), "_")

	return strings.ToLower(trimmed)
}

func newPlanID(sessionName string) string {
	timeStamp := time.Now().UTC().Format("20060102T150405")
	sessionSlug := cleanName(sessionName)

	return fmt.Sprintf("%s-%s-%s", timeStamp, sessionSlug, utils.RandID(8))
}
