package api

import (
	"context"
	"encoding/json"

	"github.com/mattermost/mattermost-plugin-playbooks/server/app"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/pkg/errors"
	"gopkg.in/guregu/null.v4"
)

type PlaybookResolver struct {
	app.Playbook
}

func (r *PlaybookResolver) IsFavorite(ctx context.Context) (bool, error) {
	c, err := getContext(ctx)
	if err != nil {
		return false, err
	}
	userID := c.r.Header.Get("Mattermost-User-ID")

	isFavorite, err := c.categoryService.IsItemFavorite(
		app.CategoryItem{
			ItemID: r.ID,
			Type:   app.PlaybookItemType,
		},
		r.TeamID,
		userID,
	)
	if err != nil {
		return false, errors.Wrap(err, "can't determine if item is favorite or not")
	}

	return isFavorite, nil
}

func (r *PlaybookResolver) DeleteAt() float64 {
	return float64(r.Playbook.DeleteAt)
}

func (r *PlaybookResolver) RetrospectiveReminderIntervalSeconds() float64 {
	return float64(r.Playbook.RetrospectiveReminderIntervalSeconds)
}

func (r *PlaybookResolver) ReminderTimerDefaultSeconds() float64 {
	return float64(r.Playbook.ReminderTimerDefaultSeconds)
}

func (r *PlaybookResolver) Metrics() []*MetricConfigResolver {
	metricConfigResolvers := make([]*MetricConfigResolver, 0, len(r.Playbook.Metrics))
	for _, metricConfig := range r.Playbook.Metrics {
		metricConfigResolvers = append(metricConfigResolvers, &MetricConfigResolver{metricConfig})
	}

	return metricConfigResolvers
}

type MetricConfigResolver struct {
	app.PlaybookMetricConfig
}

func (r *MetricConfigResolver) Target() *int32 {
	if r.PlaybookMetricConfig.Target.Valid {
		intvalue := int32(r.PlaybookMetricConfig.Target.ValueOrZero())
		return &intvalue
	}
	return nil
}

func (r *PlaybookResolver) Checklists() []*ChecklistResolver {
	checklistResolvers := make([]*ChecklistResolver, 0, len(r.Playbook.Checklists))
	for _, checklist := range r.Playbook.Checklists {
		checklistResolvers = append(checklistResolvers, &ChecklistResolver{checklist})
	}

	return checklistResolvers
}

type ChecklistResolver struct {
	app.Checklist
}

func (r *ChecklistResolver) Items() []*ChecklistItemResolver {
	checklistItemResolvers := make([]*ChecklistItemResolver, 0, len(r.Checklist.Items))
	for _, items := range r.Checklist.Items {
		checklistItemResolvers = append(checklistItemResolvers, &ChecklistItemResolver{items})
	}

	return checklistItemResolvers
}

type ChecklistItemResolver struct {
	app.ChecklistItem
}

func (r *ChecklistItemResolver) StateModified() float64 {
	return float64(r.ChecklistItem.StateModified)
}

func (r *ChecklistItemResolver) AssigneeModified() float64 {
	return float64(r.ChecklistItem.AssigneeModified)
}

func (r *ChecklistItemResolver) CommandLastRun() float64 {
	return float64(r.ChecklistItem.CommandLastRun)
}

func (r *ChecklistItemResolver) DueDate() float64 {
	return float64(r.ChecklistItem.DueDate)
}

// RunMutationCollection hold all mutation functions for a playbookRun
type PlaybookMutationCollection struct {
}

func (r *PlaybookMutationCollection) UpdatePlaybook(ctx context.Context, args struct {
	ID      string
	Updates struct {
		Title                                *string
		Description                          *string
		Public                               *bool
		CreatePublicPlaybookRun              *bool
		ReminderMessageTemplate              *string
		ReminderTimerDefaultSeconds          *float64
		StatusUpdateEnabled                  *bool
		InvitedUserIDs                       *[]string
		InvitedGroupIDs                      *[]string
		InviteUsersEnabled                   *bool
		DefaultOwnerID                       *string
		DefaultOwnerEnabled                  *bool
		BroadcastChannelIDs                  *[]string
		BroadcastEnabled                     *bool
		WebhookOnCreationURLs                *[]string
		WebhookOnCreationEnabled             *bool
		MessageOnJoin                        *string
		MessageOnJoinEnabled                 *bool
		RetrospectiveReminderIntervalSeconds *float64
		RetrospectiveTemplate                *string
		RetrospectiveEnabled                 *bool
		WebhookOnStatusUpdateURLs            *[]string
		WebhookOnStatusUpdateEnabled         *bool
		SignalAnyKeywords                    *[]string
		SignalAnyKeywordsEnabled             *bool
		CategorizeChannelEnabled             *bool
		CategoryName                         *string
		RunSummaryTemplateEnabled            *bool
		RunSummaryTemplate                   *string
		ChannelNameTemplate                  *string
		Checklists                           *[]UpdateChecklist
		IsFavorite                           *bool
	}
}) (string, error) {
	c, err := getContext(ctx)
	if err != nil {
		return "", err
	}
	userID := c.r.Header.Get("Mattermost-User-ID")

	currentPlaybook, err := c.playbookService.Get(args.ID)
	if err != nil {
		return "", err
	}

	if currentPlaybook.DeleteAt != 0 {
		return "", errors.New("archived playbooks can not be modified")
	}

	if err := c.permissions.PlaybookManageProperties(userID, currentPlaybook); err != nil {
		return "", err
	}

	setmap := map[string]interface{}{}
	addToSetmap(setmap, "Title", args.Updates.Title)
	addToSetmap(setmap, "Description", args.Updates.Description)
	if args.Updates.Public != nil {
		if *args.Updates.Public {
			if err := c.permissions.PlaybookMakePrivate(userID, currentPlaybook); err != nil {
				return "", errors.Wrap(err, "attempted to make playbook private without permissions")
			}
		} else {
			if err := c.permissions.PlaybookMakePublic(userID, currentPlaybook); err != nil {
				return "", errors.Wrap(err, "attempted to make playbook public without permissions")
			}
		}
		if c.licenceChecker.PlaybookAllowed(*args.Updates.Public) {
			return "", errors.Wrapf(app.ErrLicensedFeature, "the playbook is not valid with the current license")
		}
		addToSetmap(setmap, "Public", args.Updates.Public)
	}
	addToSetmap(setmap, "CreatePublicIncident", args.Updates.CreatePublicPlaybookRun)
	addToSetmap(setmap, "ReminderMessageTemplate", args.Updates.ReminderMessageTemplate)
	addToSetmap(setmap, "ReminderTimerDefaultSeconds", args.Updates.ReminderTimerDefaultSeconds)
	addToSetmap(setmap, "StatusUpdateEnabled", args.Updates.StatusUpdateEnabled)

	if args.Updates.InvitedUserIDs != nil {
		filteredInvitedUserIDs := c.permissions.FilterInvitedUserIDs(*args.Updates.InvitedUserIDs, currentPlaybook.TeamID)
		addConcatToSetmap(setmap, "ConcatenatedInvitedUserIDs", &filteredInvitedUserIDs)
	}

	if args.Updates.InvitedGroupIDs != nil {
		filteredInvitedGroupIDs := c.permissions.FilterInvitedGroupIDs(*args.Updates.InvitedGroupIDs)
		addConcatToSetmap(setmap, "ConcatenatedInvitedGroupIDs", &filteredInvitedGroupIDs)
	}

	addToSetmap(setmap, "InviteUsersEnabled", args.Updates.InviteUsersEnabled)
	if args.Updates.DefaultOwnerID != nil {
		if !c.pluginAPI.User.HasPermissionToTeam(*args.Updates.DefaultOwnerID, currentPlaybook.TeamID, model.PermissionViewTeam) {
			return "", errors.Wrap(app.ErrNoPermissions, "default owner can't view team")
		}
		addToSetmap(setmap, "DefaultCommanderID", args.Updates.DefaultOwnerID)
	}
	addToSetmap(setmap, "DefaultCommanderEnabled", args.Updates.DefaultOwnerEnabled)

	if args.Updates.BroadcastChannelIDs != nil {
		if err := c.permissions.NoAddedBroadcastChannelsWithoutPermission(userID, *args.Updates.BroadcastChannelIDs, currentPlaybook.BroadcastChannelIDs); err != nil {
			return "", err
		}
		addConcatToSetmap(setmap, "ConcatenatedBroadcastChannelIDs", args.Updates.BroadcastChannelIDs)
	}

	addToSetmap(setmap, "BroadcastEnabled", args.Updates.BroadcastEnabled)
	if args.Updates.WebhookOnCreationURLs != nil {
		if err := app.ValidateWebhookURLs(*args.Updates.WebhookOnCreationURLs); err != nil {
			return "", err
		}
		addConcatToSetmap(setmap, "ConcatenatedWebhookOnCreationURLs", args.Updates.WebhookOnCreationURLs)
	}
	addToSetmap(setmap, "WebhookOnCreationEnabled", args.Updates.WebhookOnCreationEnabled)
	addToSetmap(setmap, "MessageOnJoin", args.Updates.MessageOnJoin)
	addToSetmap(setmap, "MessageOnJoinEnabled", args.Updates.MessageOnJoinEnabled)
	addToSetmap(setmap, "RetrospectiveReminderIntervalSeconds", args.Updates.RetrospectiveReminderIntervalSeconds)
	addToSetmap(setmap, "RetrospectiveTemplate", args.Updates.RetrospectiveTemplate)
	addToSetmap(setmap, "RetrospectiveEnabled", args.Updates.RetrospectiveEnabled)
	if args.Updates.WebhookOnStatusUpdateURLs != nil {
		if err := app.ValidateWebhookURLs(*args.Updates.WebhookOnStatusUpdateURLs); err != nil {
			return "", err
		}
		addConcatToSetmap(setmap, "ConcatenatedWebhookOnStatusUpdateURLs", args.Updates.WebhookOnStatusUpdateURLs)
	}
	addToSetmap(setmap, "WebhookOnStatusUpdateEnabled", args.Updates.WebhookOnStatusUpdateEnabled)
	if args.Updates.SignalAnyKeywords != nil {
		validSignalAnyKeywords := app.ProcessSignalAnyKeywords(*args.Updates.SignalAnyKeywords)
		addConcatToSetmap(setmap, "ConcatenatedSignalAnyKeywords", &validSignalAnyKeywords)
	}
	addToSetmap(setmap, "SignalAnyKeywordsEnabled", args.Updates.SignalAnyKeywordsEnabled)
	addToSetmap(setmap, "CategorizeChannelEnabled", args.Updates.CategorizeChannelEnabled)
	if args.Updates.CategoryName != nil {
		if err := app.ValidateCategoryName(*args.Updates.CategoryName); err != nil {
			return "", err
		}
		addToSetmap(setmap, "CategoryName", args.Updates.CategoryName)
	}
	addToSetmap(setmap, "RunSummaryTemplateEnabled", args.Updates.RunSummaryTemplateEnabled)
	addToSetmap(setmap, "RunSummaryTemplate", args.Updates.RunSummaryTemplate)
	addToSetmap(setmap, "ChannelNameTemplate", args.Updates.ChannelNameTemplate)

	// Not optimal graphql. Stopgap measure. Should be updated seperately.
	if args.Updates.Checklists != nil {
		checklistsJSON, err := json.Marshal(args.Updates.Checklists)
		if err != nil {
			return "", errors.Wrapf(err, "failed to marshal checklist in graphql json for playbook id: '%s'", args.ID)
		}
		setmap["ChecklistsJSON"] = checklistsJSON
	}

	if len(setmap) > 0 {
		if err := c.playbookStore.GraphqlUpdate(args.ID, setmap); err != nil {
			return "", err
		}
	}

	if args.Updates.IsFavorite != nil {
		if *args.Updates.IsFavorite {
			if err := c.categoryService.AddFavorite(
				app.CategoryItem{
					ItemID: currentPlaybook.ID,
					Type:   app.PlaybookItemType,
				},
				currentPlaybook.TeamID,
				userID,
			); err != nil {
				return "", err
			}
		} else {
			if err := c.categoryService.DeleteFavorite(
				app.CategoryItem{
					ItemID: currentPlaybook.ID,
					Type:   app.PlaybookItemType,
				},
				currentPlaybook.TeamID,
				userID,
			); err != nil {
				return "", err
			}
		}
	}

	return args.ID, nil
}

func (r *PlaybookMutationCollection) AddPlaybookMember(ctx context.Context, args struct {
	PlaybookID string
	UserID     string
}) (string, error) {
	c, err := getContext(ctx)
	if err != nil {
		return "", err
	}
	userID := c.r.Header.Get("Mattermost-User-ID")

	currentPlaybook, err := c.playbookService.Get(args.PlaybookID)
	if err != nil {
		return "", err
	}

	if currentPlaybook.DeleteAt != 0 {
		return "", errors.New("archived playbooks can not be modified")
	}

	if err := c.permissions.PlaybookManageMembers(userID, currentPlaybook); err != nil {
		return "", errors.Wrap(err, "attempted to modify members without permissions")
	}

	if err := c.playbookStore.AddPlaybookMember(args.PlaybookID, args.UserID); err != nil {
		return "", errors.Wrap(err, "unable to add playbook member")
	}

	return "", nil
}

func (r *PlaybookMutationCollection) RemovePlaybookMember(ctx context.Context, args struct {
	PlaybookID string
	UserID     string
}) (string, error) {
	c, err := getContext(ctx)
	if err != nil {
		return "", err
	}
	userID := c.r.Header.Get("Mattermost-User-ID")

	currentPlaybook, err := c.playbookService.Get(args.PlaybookID)
	if err != nil {
		return "", err
	}

	if currentPlaybook.DeleteAt != 0 {
		return "", errors.New("archived playbooks can not be modified")
	}

	if err := c.permissions.PlaybookManageMembers(userID, currentPlaybook); err != nil {
		return "", errors.Wrap(err, "attempted to modify members without permissions")
	}

	if err := c.playbookStore.RemovePlaybookMember(args.PlaybookID, args.UserID); err != nil {
		return "", errors.Wrap(err, "unable to remove playbook member")
	}

	return "", nil
}

func (r *PlaybookMutationCollection) AddMetric(ctx context.Context, args struct {
	PlaybookID  string
	Title       string
	Description string
	Type        string
	Target      *float64
}) (string, error) {
	c, err := getContext(ctx)
	if err != nil {
		return "", err
	}
	userID := c.r.Header.Get("Mattermost-User-ID")

	currentPlaybook, err := c.playbookService.Get(args.PlaybookID)
	if err != nil {
		return "", err
	}

	if currentPlaybook.DeleteAt != 0 {
		return "", errors.New("archived playbooks can not be modified")
	}

	if err := c.permissions.PlaybookManageProperties(userID, currentPlaybook); err != nil {
		return "", err
	}

	var target null.Int
	if args.Target == nil {
		target = null.NewInt(0, false)
	} else {
		target = null.IntFrom(int64(*args.Target))
	}

	if err := c.playbookStore.AddMetric(args.PlaybookID, app.PlaybookMetricConfig{
		Title:       args.Title,
		Description: args.Description,
		Type:        args.Type,
		Target:      target,
	}); err != nil {
		return "", err
	}

	return args.PlaybookID, nil
}

func (r *PlaybookMutationCollection) UpdateMetric(ctx context.Context, args struct {
	ID          string
	Title       *string
	Description *string
	Target      *float64
}) (string, error) {
	c, err := getContext(ctx)
	if err != nil {
		return "", err
	}
	userID := c.r.Header.Get("Mattermost-User-ID")

	currentMetric, err := c.playbookStore.GetMetric(args.ID)
	if err != nil {
		return "", err
	}

	currentPlaybook, err := c.playbookService.Get(currentMetric.PlaybookID)
	if err != nil {
		return "", err
	}

	if currentPlaybook.DeleteAt != 0 {
		return "", errors.New("archived playbooks can not be modified")
	}

	if err := c.permissions.PlaybookManageProperties(userID, currentPlaybook); err != nil {
		return "", err
	}

	setmap := map[string]interface{}{}
	addToSetmap(setmap, "Title", args.Title)
	addToSetmap(setmap, "Description", args.Description)
	if args.Target != nil {
		setmap["Target"] = null.IntFrom(int64(*args.Target))
	}
	if len(setmap) > 0 {
		if err := c.playbookStore.UpdateMetric(args.ID, setmap); err != nil {
			return "", err
		}
	}

	return args.ID, nil
}

func (r *PlaybookMutationCollection) DeleteMetric(ctx context.Context, args struct {
	ID string
}) (string, error) {
	c, err := getContext(ctx)
	if err != nil {
		return "", err
	}
	userID := c.r.Header.Get("Mattermost-User-ID")

	currentMetric, err := c.playbookStore.GetMetric(args.ID)
	if err != nil {
		return "", err
	}

	currentPlaybook, err := c.playbookService.Get(currentMetric.PlaybookID)
	if err != nil {
		return "", err
	}

	if err := c.permissions.PlaybookManageProperties(userID, currentPlaybook); err != nil {
		return "", err
	}

	if err := c.playbookStore.DeleteMetric(args.ID); err != nil {
		return "", err
	}

	return args.ID, nil
}
