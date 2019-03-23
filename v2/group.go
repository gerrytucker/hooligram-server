package v2

import (
	"github.com/hooligram/hooligram-server/actions"
	"github.com/hooligram/hooligram-server/clients"
	"github.com/hooligram/hooligram-server/db"
	"github.com/hooligram/hooligram-server/utils"
)

func handleGroupAddMemberRequest(client *clients.Client, action *actions.Action) *actions.Action {
	if !client.IsSignedIn() {
		utils.LogBody(v2Tag, "client not signed in")
		failure := actions.CreateGroupAddMemberFailure([]string{"not signed in"})
		client.WriteJSON(failure)
		return failure
	}

	groupID, ok := action.Payload["group_id"].(float64)
	if !ok {
		failure := actions.CreateGroupAddMemberFailure([]string{"group_id is missing"})
		client.WriteJSON(failure)
		return failure
	}

	memberID, ok := action.Payload["member_id"].(float64)
	if !ok {
		failure := actions.CreateGroupAddMemberFailure([]string{"member_id is missing"})
		client.WriteJSON(failure)
		return failure
	}

	if !db.IsClientInMessageGroup(client.GetID(), int(groupID)) {
		failure := actions.CreateGroupAddMemberFailure([]string{"not allowed"})
		client.WriteJSON(failure)
		return failure
	}

	err := db.CreateMessageGroupMembers(int(groupID), []int{int(memberID)})
	if err != nil {
		utils.LogBody(v2Tag, "error adding new message group member. "+err.Error())
		failure := actions.CreateGroupAddMemberFailure([]string{"server error"})
		client.WriteJSON(failure)
		return failure
	}

	success := actions.CreateGroupAddMemberSuccess()
	client.WriteJSON(success)
	return success
}

func handleGroupCreateRequest(client *clients.Client, action *actions.Action) *actions.Action {
	groupName, ok := action.Payload["name"].(string)
	if !ok {
		failure := actions.CreateGroupCreateFailure([]string{"name not in payload"})
		client.WriteJSON(failure)
		return failure
	}

	memberIDsPayload, ok := action.Payload["member_ids"].([]interface{})
	if !ok {
		failure := actions.CreateGroupCreateFailure([]string{"member_ids not in payload"})
		client.WriteJSON(failure)
		return failure
	}

	memberIDs := make([]int, len(memberIDsPayload))

	for i, memberID := range memberIDsPayload {
		memberIDs[i] = int(memberID.(float64))
	}

	if len(memberIDs) < 2 {
		failure := actions.CreateGroupCreateFailure([]string{"need at least two members"})
		client.WriteJSON(failure)
		return failure
	}

	if !utils.ContainsID(memberIDs, client.GetID()) {
		failure := actions.CreateGroupCreateFailure([]string{"include group creator in member_ids"})
		client.WriteJSON(failure)
		return failure
	}

	messageGroup, err := db.CreateMessageGroup(groupName, memberIDs)
	if err != nil {
		utils.LogBody(v2Tag, "error creating message group. "+err.Error())
		failure := actions.CreateGroupCreateFailure([]string{"server error"})
		client.WriteJSON(failure)
		return failure
	}

	success := actions.CreateGroupCreateSuccess(
		messageGroup.ID,
		messageGroup.Name,
		messageGroup.MemberIDs,
		messageGroup.DateCreated,
	)
	client.WriteJSON(success)
	return success
}

func handleGroupLeaveRequest(client *clients.Client, action *actions.Action) *actions.Action {
	if !client.IsSignedIn() {
		utils.LogBody(v2Tag, "client not signed in")
		failure := actions.CreateGroupLeaveFailure([]string{"not signed in"})
		client.WriteJSON(failure)
		return failure
	}

	groupID, ok := action.Payload["group_id"].(float64)
	if !ok {
		failure := actions.CreateGroupLeaveFailure(([]string{"group_id is missing"}))
		client.WriteJSON(failure)
		return failure
	}

	if !db.IsClientInMessageGroup(client.GetID(), int(groupID)) {
		failure := actions.CreateGroupLeaveFailure(([]string{"not in group"}))
		client.WriteJSON(failure)
		return failure
	}

	err := db.DeleteMessageGroupMembers(int(groupID), []int{client.GetID()})
	if err != nil {
		utils.LogBody(v2Tag, "error removing client from message group. "+err.Error())
		failure := actions.CreateGroupLeaveFailure(([]string{"server error"}))
		client.WriteJSON(failure)
		return failure
	}

	success := actions.CreateGroupLeaveSuccess()
	client.WriteJSON(success)
	return success
}
