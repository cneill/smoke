package plan_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cneill/smoke/pkg/config"
	"github.com/cneill/smoke/pkg/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreBucketNaming(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	projectPath := filepath.Join(t.TempDir(), "My Project!")
	plansDir, err := config.GetPlansDirPath()
	require.NoError(t, err)

	bucket, err := plan.NewProjectBucket(plansDir, projectPath)
	require.NoError(t, err)

	assert.Equal(t, "my_project", bucket.Slug())
	assert.Equal(t, filepath.Join(plansDir, bucket.Name()), bucket.Path())
}

func TestStoreLazyManagerCreatesFilesOnFirstWrite(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	store, err := plan.NewStore(t.TempDir())
	require.NoError(t, err)

	manager, metadata, err := store.NewLazyManager("main")
	require.NoError(t, err)

	assert.NoDirExists(t, metadata.BucketPath)
	assert.NoFileExists(t, metadata.LogPath)
	assert.NoFileExists(t, store.Bucket.PlanMetadataPath(metadata.PlanID))

	item := plan.NewTaskItem("task1", "do the thing", plan.OperationAdd).ToUnion()
	require.NoError(t, manager.HandleItem(item))

	assert.DirExists(t, metadata.BucketPath)
	assert.FileExists(t, metadata.LogPath)
	assert.FileExists(t, store.Bucket.PlanMetadataPath(metadata.PlanID))

	readMetadata, err := store.ReadMetadata(metadata.PlanID)
	require.NoError(t, err)
	assert.Equal(t, metadata.PlanID, readMetadata.PlanID)
	assert.Equal(t, metadata.LogPath, readMetadata.LogPath)
	assert.Equal(t, store.Bucket.ProjectPath, readMetadata.ProjectPath)
}

func TestStoreListAndOpenCurrentProjectPlans(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	store, err := plan.NewStore(filepath.Join(t.TempDir(), "project"))
	require.NoError(t, err)

	oldManager, oldMetadata, err := store.NewLazyManager("main")
	require.NoError(t, err)
	require.NoError(t, oldManager.HandleItem(plan.NewTaskItem("old", "old plan", plan.OperationAdd).ToUnion()))

	oldMetadata.LastUsedAt = time.Now().Add(-time.Hour)
	require.NoError(t, store.WriteMetadata(oldMetadata))

	newManager, newMetadata, err := store.NewLazyManager("main")
	require.NoError(t, err)
	require.NoError(t, newManager.HandleItem(plan.NewTaskItem("new", "new plan", plan.OperationAdd).ToUnion()))

	newMetadata.LastUsedAt = time.Now()
	require.NoError(t, store.WriteMetadata(newMetadata))

	otherStore, err := plan.NewStore(filepath.Join(t.TempDir(), "other"))
	require.NoError(t, err)
	otherManager, _, err := otherStore.NewLazyManager("main")
	require.NoError(t, err)
	require.NoError(t, otherManager.HandleItem(plan.NewTaskItem("other", "other plan", plan.OperationAdd).ToUnion()))

	plans, err := store.List()
	require.NoError(t, err)
	require.Len(t, plans, 2)
	assert.Equal(t, newMetadata.PlanID, plans[0].PlanID)
	assert.Equal(t, oldMetadata.PlanID, plans[1].PlanID)

	opened, openedMetadata, err := store.Open(oldMetadata.PlanID)
	require.NoError(t, err)
	assert.Equal(t, oldMetadata.PlanID, openedMetadata.PlanID)
	assert.NotNil(t, opened.GetItemByID("old"))
	assert.Nil(t, opened.GetItemByID("new"))
}

func TestLazyManagerFromPathDoesNotCreateBeforeWrite(t *testing.T) {
	t.Parallel()

	metadata := plan.Metadata{LogPath: filepath.Join(t.TempDir(), "nested", "plan.jsonl")}

	manager, err := plan.LazyManagerFromMetadata(metadata)
	require.NoError(t, err)

	_, err = os.Stat(metadata.LogPath)
	assert.True(t, os.IsNotExist(err))

	require.Empty(t, manager.AllItems())
}
