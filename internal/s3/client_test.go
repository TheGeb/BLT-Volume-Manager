package s3

import (
	"strings"
	"testing"

	s3sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func TestCheckDeleteObjectsResponse_PartialFailure(t *testing.T) {
	t.Parallel()
	result := &s3sdk.DeleteObjectsOutput{
		Errors: []types.Error{
			{Key: awsStr("obj2"), Code: awsStr("AccessDenied"), Message: awsStr("Access Denied")},
		},
	}
	err := checkDeleteObjectsResponse(result, "test-bucket", "prefix/")
	if err == nil {
		t.Fatal("expected error for partial failure, got nil")
	}
	if !strings.Contains(err.Error(), "partial failure") {
		t.Errorf("expected 'partial failure' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Access Denied") {
		t.Errorf("expected 'Access Denied' in error, got: %v", err)
	}
}

func TestCheckDeleteObjectsResponse_Success(t *testing.T) {
	t.Parallel()
	result := &s3sdk.DeleteObjectsOutput{
		Deleted: []types.DeletedObject{
			{Key: awsStr("obj1")},
		},
	}
	err := checkDeleteObjectsResponse(result, "test-bucket", "prefix/")
	if err != nil {
		t.Fatalf("expected nil for successful deletion, got: %v", err)
	}
}

func TestCheckDeleteObjectsResponse_Nil(t *testing.T) {
	t.Parallel()
	err := checkDeleteObjectsResponse(nil, "test-bucket", "prefix/")
	if err != nil {
		t.Fatalf("expected nil for nil result, got: %v", err)
	}
}

func TestCheckDeleteObjectsResponse_MultipleErrors(t *testing.T) {
	t.Parallel()
	result := &s3sdk.DeleteObjectsOutput{
		Errors: []types.Error{
			{Key: awsStr("obj1"), Code: awsStr("AccessDenied"), Message: awsStr("Access Denied")},
			{Key: awsStr("obj2"), Code: awsStr("InternalError"), Message: awsStr("We encountered an internal error")},
		},
	}
	err := checkDeleteObjectsResponse(result, "test-bucket", "prefix/")
	if err == nil {
		t.Fatal("expected error for multiple errors")
	}
	if !strings.Contains(err.Error(), "obj1") || !strings.Contains(err.Error(), "obj2") {
		t.Errorf("expected both errors mentioned, got: %v", err)
	}
}

func awsStr(s string) *string { return &s }
