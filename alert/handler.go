package alert

import (
	"easy-api-prom-alert-sms/config"
	"easy-api-prom-alert-sms/logging"
	"io"

	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/prometheus/alertmanager/template"
)

func (alertSender *AlertSender) AlertHandler(resp http.ResponseWriter, req *http.Request) {
	var alertData template.Data

	if err := json.NewDecoder(req.Body).Decode(&alertData); err != nil {
		logging.Log(logging.Error, "failed to parse content : %s", err.Error())
		http.Error(resp, err.Error(), http.StatusBadRequest)
		return
	}

	alertSender.setData(&alertData)

	go func() {
		if err := alertSender.sendAlert(); err != nil {
			logging.Log(logging.Error, "failed to send alert : %s", err.Error())
			http.Error(resp, err.Error(), http.StatusInternalServerError)
			return
		}
	}()

	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(http.StatusNoContent)
}

func (alertSender *AlertSender) getPostAndQueryParams(member string, message string) (map[string]string, string) {
	postBody := map[string]string{
		alertSender.config.EasyAPIPromAlertSMS.Provider.Parameters.Message.ParamName: message,
	}
	queryBody := ""

	if alertSender.config.EasyAPIPromAlertSMS.Provider.Parameters.From.ParamMethod == config.PostMethod {
		postBody[alertSender.config.EasyAPIPromAlertSMS.Provider.Parameters.From.ParamName] = alertSender.config.EasyAPIPromAlertSMS.Provider.Parameters.From.ParamValue
	} else {
		queryBody = "?" + alertSender.config.EasyAPIPromAlertSMS.Provider.Parameters.From.ParamName + "=" + alertSender.config.EasyAPIPromAlertSMS.Provider.Parameters.From.ParamValue
	}

	if alertSender.config.EasyAPIPromAlertSMS.Provider.Parameters.To.ParamMethod == config.PostMethod {
		postBody[alertSender.config.EasyAPIPromAlertSMS.Provider.Parameters.To.ParamName] = member
	} else if queryBody != "" {
		queryBody = queryBody + "&" + alertSender.config.EasyAPIPromAlertSMS.Provider.Parameters.To.ParamName + "=" + member
	} else {
		queryBody = "?" + alertSender.config.EasyAPIPromAlertSMS.Provider.Parameters.To.ParamName + "=" + member
	}

	return postBody, queryBody
}

func (alertSender *AlertSender) sendAlert() error {
	for _, alert := range alertSender.data.Alerts {
		alertMsg := alertSender.getMsgFromAlert(alert)
		recipientName := alertSender.getRecipientFromAlert(alert)
		members := alertSender.getRecipientMembers(recipientName)

		for _, member := range members {

			var builder strings.Builder
			body, query := alertSender.getPostAndQueryParams(member, alertMsg)
			if err := json.NewEncoder(&builder).Encode(body); err != nil {
				return err
			}

			if alertSender.config.EasyAPIPromAlertSMS.Simulation {
				logging.Log(logging.Info, builder.String())
			} else {
				if err := sendSMSFromProviderApi(alertSender.config, builder.String(), query); err != nil {
					logging.Log(logging.Error, err.Error())
				}
			}
		}
	}

	return nil
}

func sendSMSFromProviderApi(config *config.Config, body string, query string) error {
	client := &http.Client{
		Timeout: config.EasyAPIPromAlertSMS.Provider.Timeout,
	}

	providerUrl := config.EasyAPIPromAlertSMS.Provider.Url + query
	req, err := http.NewRequest("POST", providerUrl, strings.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if config.EasyAPIPromAlertSMS.Provider.Authentication.Enabled {
		req.Header.Set("Authorization", config.EasyAPIPromAlertSMS.Provider.Authentication.Authorization.Type+" "+config.EasyAPIPromAlertSMS.Provider.Authentication.Authorization.Credential)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	var respBody []byte
	respBody, err = io.ReadAll(resp.Body)
	if err != nil {
		logging.Log(logging.Error, "Failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed with status : %s", resp.Status)
	}

	logging.Log(logging.Info, "successful send request with body %s", body)
	logging.Log(logging.Info, "response body %s", string(respBody))
	return nil
}
