package prototest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	zee6dov1 "github.com/DeepakDP5/zee6do-server/gen/zee6do/v1"
)

// TestUserInCommon verifies that the User message is defined in common.proto
// (not auth_service.proto) and can be used across services.
func TestUserInCommon(t *testing.T) {
	user := &zee6dov1.User{
		Id:        "user-123",
		Phone:     "+14155552671",
		Email:     "test@example.com",
		Name:      "Test User",
		AvatarUrl: "https://example.com/avatar.png",
		CreatedAt: timestamppb.Now(),
	}

	// Verify User can be embedded in auth responses
	verifyResp := &zee6dov1.VerifyOTPResponse{User: user}
	assert.Equal(t, "user-123", verifyResp.GetUser().GetId())

	// Verify User can be embedded in user profiles
	profile := &zee6dov1.UserProfile{User: user}
	assert.Equal(t, "user-123", profile.GetUser().GetId())

	// Verify User can be embedded in social login responses
	socialResp := &zee6dov1.SocialLoginResponse{User: user}
	assert.Equal(t, "user-123", socialResp.GetUser().GetId())
}

// TestNotificationSettingsInCommon verifies that NotificationSettings is defined
// in common.proto and usable by both user_service and notification_service.
func TestNotificationSettingsInCommon(t *testing.T) {
	settings := &zee6dov1.NotificationSettings{
		MorningBriefing: true,
		Reminders:       true,
		Deadlines:       true,
		ConnectorAlerts: false,
		DailySummary:    true,
		QuietHoursStart: "22:00",
		QuietHoursEnd:   "07:00",
	}

	// Verify NotificationSettings is usable in UserPreferences
	prefs := &zee6dov1.UserPreferences{NotificationSettings: settings}
	assert.True(t, prefs.GetNotificationSettings().GetMorningBriefing())
	assert.Equal(t, "22:00", prefs.GetNotificationSettings().GetQuietHoursStart())

	// Verify NotificationSettings is usable in notification service responses
	getPrefsResp := &zee6dov1.GetPreferencesResponse{Preferences: settings}
	assert.True(t, getPrefsResp.GetPreferences().GetReminders())

	updatePrefsResp := &zee6dov1.UpdatePreferencesResponse{Preferences: settings}
	assert.True(t, updatePrefsResp.GetPreferences().GetDeadlines())
}

// TestDateRangeInCommon verifies that DateRange is defined in common.proto
// and reusable across analytics and scheduler services.
func TestDateRangeInCommon(t *testing.T) {
	dr := &zee6dov1.DateRange{
		Start: timestamppb.Now(),
		End:   timestamppb.Now(),
	}

	// Verify DateRange is usable in analytics requests
	metricsReq := &zee6dov1.GetMetricsRequest{DateRange: dr}
	require.NotNil(t, metricsReq.GetDateRange())
	assert.NotNil(t, metricsReq.GetDateRange().GetStart())

	timeReq := &zee6dov1.GetTimeBreakdownRequest{DateRange: dr}
	require.NotNil(t, timeReq.GetDateRange())

	narrativeReq := &zee6dov1.GetAINarrativeRequest{DateRange: dr}
	require.NotNil(t, narrativeReq.GetDateRange())

	// Verify DateRange is usable in scheduler requests
	agendaReq := &zee6dov1.GetAgendaViewRequest{DateRange: dr}
	require.NotNil(t, agendaReq.GetDateRange())
	assert.NotNil(t, agendaReq.GetDateRange().GetStart())
	assert.NotNil(t, agendaReq.GetDateRange().GetEnd())
}

// TestSuggestionActionPayloadIsStruct verifies that Suggestion.action_payload
// uses google.protobuf.Struct instead of a plain string.
func TestSuggestionActionPayloadIsStruct(t *testing.T) {
	payload, err := structpb.NewStruct(map[string]interface{}{
		"action":   "reschedule",
		"task_id":  "task-456",
		"new_date": "2025-01-15",
	})
	require.NoError(t, err)

	suggestion := &zee6dov1.Suggestion{
		Id:            "sug-1",
		Type:          zee6dov1.SuggestionType_SUGGESTION_TYPE_RESCHEDULE,
		Message:       "Consider rescheduling this task",
		TaskId:        "task-456",
		ActionPayload: payload,
	}

	// Verify the struct is properly set
	ap := suggestion.GetActionPayload()
	require.NotNil(t, ap)
	fields := ap.GetFields()
	assert.Equal(t, "reschedule", fields["action"].GetStringValue())
	assert.Equal(t, "task-456", fields["task_id"].GetStringValue())

	// Verify it round-trips through proto marshal/unmarshal
	data, err := proto.Marshal(suggestion)
	require.NoError(t, err)

	decoded := &zee6dov1.Suggestion{}
	err = proto.Unmarshal(data, decoded)
	require.NoError(t, err)
	assert.Equal(t, "reschedule", decoded.GetActionPayload().GetFields()["action"].GetStringValue())
}

// TestPaginationFieldExists verifies that Pagination has the expected fields.
// The validation constraint (gte:1, lte:100) is defined in the proto schema
// and enforced at runtime by the protovalidate interceptor.
func TestPaginationFieldExists(t *testing.T) {
	p := &zee6dov1.Pagination{
		PageSize:  50,
		PageToken: "next-page-token",
	}
	assert.Equal(t, int32(50), p.GetPageSize())
	assert.Equal(t, "next-page-token", p.GetPageToken())
}

// TestSendOTPRequestPhoneNumber verifies the SendOTPRequest message structure.
func TestSendOTPRequestPhoneNumber(t *testing.T) {
	req := &zee6dov1.SendOTPRequest{
		PhoneNumber: "+14155552671",
	}
	assert.Equal(t, "+14155552671", req.GetPhoneNumber())
}
