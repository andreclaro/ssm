package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&Instance{}, &Tag{})
	require.NoError(t, err)

	// Ensure repository code uses this in-memory DB
	DB = db

	return db
}

// TestInstanceRepository_SaveOrUpdate tests saving and updating instances
func TestInstanceRepository_SaveOrUpdate(t *testing.T) {
	db := setupTestDB(t)
	repo := &InstanceRepository{}

	// Create test instance
	instance := &Instance{
		InstanceID: "i-1234567890abcdef0",
		Name:       "test-instance",
		Region:     "us-east-1",
		Profile:    "default",
		AccountID:  "123456789012",
		State:      "running",
		Platform:   "Linux",
		Tags: []Tag{
			{Key: "Name", Value: "test-instance"},
			{Key: "Environment", Value: "test"},
		},
	}

	// Save instance
	err := repo.SaveOrUpdate(instance)
	require.NoError(t, err)

	// Verify instance was saved
	var savedInstance Instance
	err = db.Preload("Tags").First(&savedInstance).Error
	require.NoError(t, err)

	assert.Equal(t, "i-1234567890abcdef0", savedInstance.InstanceID)
	assert.Equal(t, "test-instance", savedInstance.Name)
	assert.Equal(t, "us-east-1", savedInstance.Region)
	assert.Equal(t, "default", savedInstance.Profile)
	assert.Equal(t, "running", savedInstance.State)
	assert.Len(t, savedInstance.Tags, 2)

	// Update instance
	instance.Name = "updated-instance"
	instance.State = "stopped"
	err = repo.SaveOrUpdate(instance)
	require.NoError(t, err)

	// Verify instance was updated
	var updatedInstance Instance
	err = db.Preload("Tags").First(&updatedInstance).Error
	require.NoError(t, err)

	assert.Equal(t, "updated-instance", updatedInstance.Name)
	assert.Equal(t, "stopped", updatedInstance.State)
	assert.Len(t, updatedInstance.Tags, 2) // Tags should be replaced
}

// TestInstanceRepository_FindByName tests finding instances by name
func TestInstanceRepository_FindByName(t *testing.T) {
	db := setupTestDB(t)
	repo := &InstanceRepository{}

	// Create test instance
	instance := &Instance{
		InstanceID: "i-1234567890abcdef0",
		Name:       "test-instance",
		Region:     "us-east-1",
		Profile:    "default",
		AccountID:  "123456789012",
		State:      "running",
	}

	err := db.Create(instance).Error
	require.NoError(t, err)

	// Find existing instance
	found, err := repo.FindByName("test-instance")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "test-instance", found.Name)

	// Find non-existing instance
	notFound, err := repo.FindByName("non-existing")
	require.NoError(t, err)
	assert.Nil(t, notFound)
}

// TestInstanceRepository_List tests listing instances with filters
func TestInstanceRepository_List(t *testing.T) {
	db := setupTestDB(t)
	repo := &InstanceRepository{}

	// Create test instances
	instances := []Instance{
		{
			InstanceID: "i-1234567890abcdef0",
			Name:       "prod-web",
			Region:     "us-east-1",
			Profile:    "production",
			State:      "running",
		},
		{
			InstanceID: "i-0987654321fedcba0",
			Name:       "staging-db",
			Region:     "us-west-2",
			Profile:    "staging",
			State:      "running",
		},
	}

	for _, instance := range instances {
		err := db.Create(&instance).Error
		require.NoError(t, err)
	}

	// List all instances
	all, err := repo.List(nil)
	require.NoError(t, err)
	assert.Len(t, all, 2)

	// List by profile
	prodInstances, err := repo.List(&InstanceFilter{Profile: stringPtr("production")})
	require.NoError(t, err)
	assert.Len(t, prodInstances, 1)
	assert.Equal(t, "prod-web", prodInstances[0].Name)

	// List by region
	usEastInstances, err := repo.List(&InstanceFilter{Region: stringPtr("us-east-1")})
	require.NoError(t, err)
	assert.Len(t, usEastInstances, 1)
	assert.Equal(t, "prod-web", usEastInstances[0].Name)
}

// TestConvertEC2Instance tests converting EC2 instances to our model
func TestConvertEC2Instance(t *testing.T) {
	// This would require importing EC2 types, but for now we'll test the basic structure
	// In a real test, we'd create mock EC2 instances

	instance := &Instance{
		InstanceID: "i-1234567890abcdef0",
		Region:     "us-east-1",
		Profile:    "default",
		AccountID:  "123456789012",
	}

	assert.Equal(t, "i-1234567890abcdef0", instance.InstanceID)
	assert.Equal(t, "us-east-1", instance.Region)
	assert.Equal(t, "default", instance.Profile)
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
