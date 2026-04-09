package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockPermissionItem implements PermissionItem interface
type MockPermissionItem struct {
	mock.Mock
}

func (m *MockPermissionItem) HasAccess(action string) bool {
	args := m.Called(action)
	return args.Bool(0)
}

// MockPermissionChecker implements PermissionChecker interface
type MockPermissionChecker struct {
	mock.Mock
}

func (m *MockPermissionChecker) ListAccountantPermission(ctx context.Context, accId uuid.UUID) ([]PermissionItem, error) {
	args := m.Called(ctx, accId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]PermissionItem), args.Error(1)
}

func (m *MockPermissionChecker) GetPermissionsForAccountant(ctx context.Context, accountantID uuid.UUID, entityID uuid.UUID) (PermissionItem, error) {
	args := m.Called(ctx, accountantID, entityID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(PermissionItem), args.Error(1)
}

// TestMethodBasedPermission_PractitionerBypass tests that practitioners bypass permission checks
func TestMethodBasedPermission_PractitionerBypass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	mockChecker := new(MockPermissionChecker)
	
	router := gin.New()
	router.GET("/test/:id", func(c *gin.Context) {
		c.Set(util.EntityIDKey, uuid.New())
		c.Set("role", util.RolePractitioner)
		c.Next()
	}, MethodBasedPermission(mockChecker), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Verify no permission checks were called
	mockChecker.AssertNotCalled(t, "GetPermissionsForAccountant")
	mockChecker.AssertNotCalled(t, "ListAccountantPermission")
}

// TestMethodBasedPermission_AccountantWithReadAccess tests accountant with read permission
func TestMethodBasedPermission_AccountantWithReadAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	mockChecker := new(MockPermissionChecker)
	mockPerm := new(MockPermissionItem)
	
	accountantID := uuid.New()
	entityID := uuid.New()
	
	mockPerm.On("HasAccess", "read").Return(true)
	mockChecker.On("GetPermissionsForAccountant", mock.Anything, accountantID, entityID).Return(mockPerm, nil)
	
	router := gin.New()
	router.GET("/test/:id", func(c *gin.Context) {
		c.Set(util.EntityIDKey, accountantID)
		c.Set("role", util.RoleAccountant)
		c.Next()
	}, MethodBasedPermission(mockChecker), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test/"+entityID.String(), nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockChecker.AssertExpectations(t)
	mockPerm.AssertExpectations(t)
}

// TestMethodBasedPermission_AccountantWithoutAccess tests accountant without permission
func TestMethodBasedPermission_AccountantWithoutAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	mockChecker := new(MockPermissionChecker)
	mockPerm := new(MockPermissionItem)
	
	accountantID := uuid.New()
	entityID := uuid.New()
	
	mockPerm.On("HasAccess", "update").Return(false)
	mockChecker.On("GetPermissionsForAccountant", mock.Anything, accountantID, entityID).Return(mockPerm, nil)
	
	router := gin.New()
	router.PUT("/test/:id", func(c *gin.Context) {
		c.Set(util.EntityIDKey, accountantID)
		c.Set("role", util.RoleAccountant)
		c.Next()
	}, MethodBasedPermission(mockChecker), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodPut, "/test/"+entityID.String(), nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	mockChecker.AssertExpectations(t)
	mockPerm.AssertExpectations(t)
}

// TestMethodBasedPermission_HTTPMethodMapping tests correct action mapping
func TestMethodBasedPermission_HTTPMethodMapping(t *testing.T) {
	tests := []struct {
		method         string
		expectedAction string
	}{
		{http.MethodGet, "read"},
		{http.MethodPost, "create"},
		{http.MethodPut, "update"},
		{http.MethodPatch, "update"},
		{http.MethodDelete, "delete"},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			
			mockChecker := new(MockPermissionChecker)
			mockPerm := new(MockPermissionItem)
			
			accountantID := uuid.New()
			entityID := uuid.New()
			
			mockPerm.On("HasAccess", tt.expectedAction).Return(true)
			mockChecker.On("GetPermissionsForAccountant", mock.Anything, accountantID, entityID).Return(mockPerm, nil)
			
			router := gin.New()
			router.Handle(tt.method, "/test/:id", func(c *gin.Context) {
				c.Set(util.EntityIDKey, accountantID)
				c.Set("role", util.RoleAccountant)
				c.Next()
			}, MethodBasedPermission(mockChecker), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest(tt.method, "/test/"+entityID.String(), nil)
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			mockPerm.AssertCalled(t, "HasAccess", tt.expectedAction)
		})
	}
}

// TestMethodBasedPermission_ListOperation tests list operations without entity ID
func TestMethodBasedPermission_ListOperation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	mockChecker := new(MockPermissionChecker)
	mockPerm := new(MockPermissionItem)
	
	accountantID := uuid.New()
	
	mockPerm.On("HasAccess", "read").Return(true)
	mockChecker.On("ListAccountantPermission", mock.Anything, accountantID).Return([]PermissionItem{mockPerm}, nil)
	
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		c.Set(util.EntityIDKey, accountantID)
		c.Set("role", util.RoleAccountant)
		c.Next()
	}, MethodBasedPermission(mockChecker), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockChecker.AssertExpectations(t)
	mockPerm.AssertExpectations(t)
}
