package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/mattermost/mattermost-server/model"
)

const (
	endPollInvalidPermission = "Only the creator of a poll is allowed to end it."

	deletePollInvalidPermission   = "Only the creator of a poll is allowed to delete it."
	deletePollFeatureNotAvailable = "This feature is only available on Mattermost v5.3."
	deletePollSuccess             = "Succefully deleted the poll."
)

func (p *MatterpollPlugin) handleVote(w http.ResponseWriter, r *http.Request) {
	var request model.PostActionIntegrationRequest
	json.NewDecoder(r.Body).Decode(&request)
	userID := request.UserId

	matches := voteRoute.FindStringSubmatch(r.URL.Path)
	pollID := matches[1]
	optionNumber, _ := strconv.Atoi(matches[2])
	response := &model.PostActionIntegrationResponse{}

	b, appErr := p.API.KVGet(pollID)
	if appErr != nil {
		response.EphemeralText = commandGenericError
		writePostActionIntegrationResponse(w, response)
		return
	}
	poll := Decode(b)
	if poll == nil {
		response.EphemeralText = commandGenericError
		writePostActionIntegrationResponse(w, response)
		return
	}

	hasVoted := poll.HasVoted(userID)
	err := poll.UpdateVote(userID, optionNumber)
	if err != nil {
		response.EphemeralText = commandGenericError
		writePostActionIntegrationResponse(w, response)
		return
	}
	appErr = p.API.KVSet(pollID, poll.Encode())
	if appErr != nil {
		response.EphemeralText = commandGenericError
		writePostActionIntegrationResponse(w, response)
		return
	}

	if hasVoted {
		response.EphemeralText = "Your vote has been updated."
	} else {
		response.EphemeralText = "Your vote has been counted."
	}
	writePostActionIntegrationResponse(w, response)
}

func (p *MatterpollPlugin) handleEndPoll(w http.ResponseWriter, r *http.Request) {
	var request model.PostActionIntegrationRequest
	json.NewDecoder(r.Body).Decode(&request)
	userID := request.UserId
	pollID := endPollRoute.FindStringSubmatch(r.URL.Path)[1]

	response := &model.PostActionIntegrationResponse{}

	b, appErr := p.API.KVGet(pollID)
	if appErr != nil {
		response.EphemeralText = commandGenericError
		writePostActionIntegrationResponse(w, response)
		return
	}
	poll := Decode(b)
	if poll == nil {
		response.EphemeralText = commandGenericError
		writePostActionIntegrationResponse(w, response)
		return
	}

	if userID != poll.Creator {
		response.EphemeralText = endPollInvalidPermission
		writePostActionIntegrationResponse(w, response)
		return
	}

	appErr = p.API.KVDelete(pollID)
	if appErr != nil {
		response.EphemeralText = commandGenericError
		writePostActionIntegrationResponse(w, response)
		return
	}

	message := "Poll is done.\n"
	for _, o := range poll.Options {
		message += fmt.Sprintf("%s:", o.Answer)
		for i := 0; i < len(o.Voter); i++ {
			user, err := p.API.GetUser(o.Voter[i])
			if err != nil {
				response.EphemeralText = commandGenericError
				writePostActionIntegrationResponse(w, response)
				return
			}
			if i+1 == len(o.Voter) && len(o.Voter) > 1 {
				message += " and"
			} else if i != 0 {
				message += ","
			}

			message += fmt.Sprintf(" @%s", user.Username)
		}
		message += "\n"
	}

	response.Update = &model.Post{
		Message: message,
	}
	writePostActionIntegrationResponse(w, response)
}

func (p *MatterpollPlugin) handleDeletePoll(w http.ResponseWriter, r *http.Request) {
	var request model.PostActionIntegrationRequest
	json.NewDecoder(r.Body).Decode(&request)
	userID := request.UserId
	pollID := deletePollRoute.FindStringSubmatch(r.URL.Path)[1]

	response := &model.PostActionIntegrationResponse{}

	b, appErr := p.API.KVGet(pollID)
	if appErr != nil {
		response.EphemeralText = commandGenericError
		writePostActionIntegrationResponse(w, response)
		return
	}
	poll := Decode(b)
	if poll == nil {
		response.EphemeralText = commandGenericError
		writePostActionIntegrationResponse(w, response)
		return
	}

	if userID != poll.Creator {
		response.EphemeralText = deletePollInvalidPermission
		writePostActionIntegrationResponse(w, response)
		return
	}

	if request.PostId == "" {
		response.EphemeralText = deletePollFeatureNotAvailable
		writePostActionIntegrationResponse(w, response)
		return
	}

	appErr = p.API.DeletePost(request.PostId)
	if appErr != nil {
		response.EphemeralText = commandGenericError
		writePostActionIntegrationResponse(w, response)
		return
	}

	appErr = p.API.KVDelete(pollID)
	if appErr != nil {
		response.EphemeralText = commandGenericError
		writePostActionIntegrationResponse(w, response)
		return
	}
	response.EphemeralText = deletePollSuccess

	writePostActionIntegrationResponse(w, response)
}

func writePostActionIntegrationResponse(w http.ResponseWriter, response *model.PostActionIntegrationResponse) {
	bytes, _ := json.Marshal(response)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(bytes)
}