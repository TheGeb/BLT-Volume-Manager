package s3

import (
	"strings"
	"testing"

	s3sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func TestDeleteObjectsErrors_PartialFailure(t *testing.T) {
	t.Parallel()
	result := &s3sdk.DeleteObjectsOutput{
		Errors: []types.Error{
			{Key: awsStr("obj2"), Code: awsStr("AccessDenied"), Message: awsStr("Access Denied")},
		},
	}
	failed := deleteObjectsErrors(result)
	if len(failed) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(failed))
	}
	if !strings.Contains(failed[0], "Access Denied") {
		t.Errorf("expected 'Access Denied' in failure, got: %v", failed[0])
	}
}

func TestDeleteObjectsErrors_Success(t *testing.T) {
	t.Parallel()
	result := &s3sdk.DeleteObjectsOutput{
		Deleted: []types.DeletedObject{
			{Key: awsStr("obj1")},
		},
	}
	if failed := deleteObjectsErrors(result); len(failed) != 0 {
		t.Fatalf("expected no failures, got: %v", failed)
	}
}

func TestDeleteObjectsErrors_Nil(t *testing.T) {
	t.Parallel()
	if failed := deleteObjectsErrors(nil); len(failed) != 0 {
		t.Fatalf("expected no failures for nil result, got: %v", failed)
	}
}

func TestDeleteObjectsErrors_MultipleErrors(t *testing.T) {
	t.Parallel()
	result := &s3sdk.DeleteObjectsOutput{
		Errors: []types.Error{
			{Key: awsStr("obj1"), Code: awsStr("AccessDenied"), Message: awsStr("Access Denied")},
			{Key: awsStr("obj2"), Code: awsStr("InternalError"), Message: awsStr("We encountered an internal error")},
		},
	}
	failed := deleteObjectsErrors(result)
	if len(failed) != 2 {
		t.Fatalf("expected 2 failures, got %d", len(failed))
	}
	if !strings.Contains(failed[0], "obj1") || !strings.Contains(failed[1], "obj2") {
		t.Errorf("expected both errors mentioned, got: %v", failed)
	}
}

func awsStr(s string) *string { return &s }
