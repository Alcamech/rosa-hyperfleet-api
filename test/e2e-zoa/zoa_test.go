package e2e_zoa_test

import (
	"encoding/json"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ZOA Trusted Actions", Ordered, func() {

	Describe("Catalog", func() {
		It("should list available trusted actions", func() {
			resp, err := apiClient.Get("/api/v0/trusted-actions", accountID)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var catalog struct {
				Items []struct {
					Name        string `json:"name"`
					Scope       string `json:"scope"`
					Type        string `json:"type"`
					Description string `json:"description"`
				} `json:"items"`
				Total int `json:"total"`
			}
			Expect(json.Unmarshal(resp.Body, &catalog)).To(Succeed())
			Expect(catalog.Items).NotTo(BeEmpty(), "Expected at least one trusted action in catalog")
			Expect(catalog.Total).To(BeNumerically(">", 0))

			GinkgoWriter.Printf("Catalog contains %d trusted actions:\n", catalog.Total)
			for _, item := range catalog.Items {
				GinkgoWriter.Printf("  - %s (scope=%s, type=%s)\n", item.Name, item.Scope, item.Type)
			}
		})

		It("should contain the get_nodes action", func() {
			resp, err := apiClient.Get("/api/v0/trusted-actions", accountID)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var catalog struct {
				Items []struct {
					Name string `json:"name"`
				} `json:"items"`
			}
			Expect(json.Unmarshal(resp.Body, &catalog)).To(Succeed())

			names := make([]string, 0, len(catalog.Items))
			for _, item := range catalog.Items {
				names = append(names, item.Name)
			}
			Expect(names).To(ContainElement("get_nodes"))
		})
	})

	Describe("Describe", func() {
		It("should describe the get_nodes action with params and required fields", func() {
			resp, err := apiClient.Get("/api/v0/trusted-actions/get_nodes", accountID)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var desc struct {
				Name           string   `json:"name"`
				Scope          string   `json:"scope"`
				Type           string   `json:"type"`
				Description    string   `json:"description"`
				RequiredFields []string `json:"required_fields"`
				Params         []struct {
					Name     string `json:"name"`
					Required bool   `json:"required"`
				} `json:"params"`
			}
			Expect(json.Unmarshal(resp.Body, &desc)).To(Succeed())
			Expect(desc.Name).To(Equal("get_nodes"))
			Expect(desc.Scope).To(Equal("kube-api"))
			Expect(desc.Type).To(Equal("read"))
			Expect(desc.RequiredFields).To(ContainElements("target_cluster", "jira"))
		})

		It("should return 404 for unknown action", func() {
			resp, err := apiClient.Get("/api/v0/trusted-actions/nonexistent_action_xyz", accountID)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})
	})

	Describe("Runs", func() {
		It("should list runs (empty or not)", func() {
			resp, err := apiClient.Get("/api/v0/trusted-actions/runs", accountID)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var list struct {
				Items   []interface{} `json:"items"`
				Total   int           `json:"total"`
				Page    int           `json:"page"`
				Limit   int           `json:"limit"`
				HasMore bool          `json:"has_more"`
			}
			Expect(json.Unmarshal(resp.Body, &list)).To(Succeed())
			Expect(list.Items).NotTo(BeNil())
			Expect(list.Page).To(Equal(1))
			Expect(list.Limit).To(BeNumerically(">", 0))
		})

		It("should return 404 for non-existent run ID", func() {
			resp, err := apiClient.Get("/api/v0/trusted-actions/runs/00000000-0000-0000-0000-000000000000", accountID)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})
	})

	Describe("Create and Execute", func() {
		It("should reject request without target_cluster", func() {
			body := map[string]interface{}{
				"jira": "ROSAENG-9999",
			}
			resp, err := apiClient.Post("/api/v0/trusted-actions/get_nodes/run", body, accountID)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))

			var errResp struct {
				Code   string `json:"code"`
				Reason string `json:"reason"`
			}
			Expect(json.Unmarshal(resp.Body, &errResp)).To(Succeed())
			Expect(errResp.Code).To(Equal("ZOA-CREATE-003"))
		})

		It("should reject request without jira ticket", func() {
			body := map[string]interface{}{
				"target_cluster": mcName,
			}
			resp, err := apiClient.Post("/api/v0/trusted-actions/get_nodes/run", body, accountID)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))

			var errResp struct {
				Code string `json:"code"`
			}
			Expect(json.Unmarshal(resp.Body, &errResp)).To(Succeed())
			Expect(errResp.Code).To(Equal("ZOA-CREATE-004"))
		})

		It("should reject request with invalid jira format", func() {
			body := map[string]interface{}{
				"target_cluster": mcName,
				"jira":           "not-a-valid-ticket",
			}
			resp, err := apiClient.Post("/api/v0/trusted-actions/get_nodes/run", body, accountID)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))

			var errResp struct {
				Code string `json:"code"`
			}
			Expect(json.Unmarshal(resp.Body, &errResp)).To(Succeed())
			Expect(errResp.Code).To(Equal("ZOA-CREATE-005"))
		})

		It("should reject request with unknown parameters", func() {
			body := map[string]interface{}{
				"target_cluster": mcName,
				"jira":           "ROSAENG-9999",
				"params": map[string]string{
					"bogus_param": "value",
				},
			}
			resp, err := apiClient.Post("/api/v0/trusted-actions/get_nodes/run", body, accountID)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))

			var errResp struct {
				Code string `json:"code"`
			}
			Expect(json.Unmarshal(resp.Body, &errResp)).To(Succeed())
			Expect(errResp.Code).To(Equal("ZOA-CREATE-006"))
		})

		It("should dispatch get_nodes and complete successfully (full wait)", func() {
			By("Submitting a get_nodes trusted action")
			body := map[string]interface{}{
				"target_cluster": mcName,
				"jira":           "ROSAENG-9999",
			}
			resp, err := apiClient.Post("/api/v0/trusted-actions/get_nodes/run", body, accountID)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

			var execution struct {
				ID            string `json:"id"`
				Action        string `json:"action"`
				TargetCluster string `json:"target_cluster"`
				Status        string `json:"status"`
				Scope         string `json:"scope"`
				Type          string `json:"type"`
			}
			Expect(json.Unmarshal(resp.Body, &execution)).To(Succeed())
			Expect(execution.ID).NotTo(BeEmpty())
			Expect(execution.Action).To(Equal("get_nodes"))
			Expect(execution.TargetCluster).To(Equal(mcName))
			Expect(execution.Status).To(Equal("pending"))
			GinkgoWriter.Printf("Execution created: id=%s status=%s\n", execution.ID, execution.Status)

			By("Polling until the execution completes")
			Eventually(func() string {
				pollResp, err := apiClient.Get(
					fmt.Sprintf("/api/v0/trusted-actions/runs/%s", execution.ID), accountID)
				if err != nil {
					GinkgoWriter.Printf("Poll error: %v\n", err)
					return "error"
				}
				var run struct {
					Status string `json:"status"`
				}
				if err := json.Unmarshal(pollResp.Body, &run); err != nil {
					return "error"
				}
				GinkgoWriter.Printf("Execution %s status: %s\n", execution.ID, run.Status)
				return run.Status
			}, "5m", "5s").Should(BeElementOf("succeeded", "failed", "timed_out"),
				"Execution should reach a terminal state")

			By("Verifying status, output_status, output content, and logs")
			finalResp, err := apiClient.Get(
				fmt.Sprintf("/api/v0/trusted-actions/runs/%s?include=output,logs", execution.ID), accountID)
			Expect(err).NotTo(HaveOccurred())
			Expect(finalResp.StatusCode).To(Equal(http.StatusOK))

			var finalExec struct {
				Status       string      `json:"status"`
				OutputStatus string      `json:"output_status"`
				Output       interface{} `json:"output"`
				Logs         string      `json:"logs"`
				Duration     int         `json:"duration_seconds"`
			}
			Expect(json.Unmarshal(finalResp.Body, &finalExec)).To(Succeed())

			Expect(finalExec.Status).To(Equal("succeeded"),
				"get_nodes should succeed on the ephemeral MC")
			Expect(finalExec.OutputStatus).To(Equal("uploaded"),
				"output should be uploaded to S3 by the uploader sidecar")

			Expect(finalExec.Output).NotTo(BeNil(), "Expected node list output from get_nodes")
			outputSlice, ok := finalExec.Output.([]interface{})
			Expect(ok).To(BeTrue(), "Output should be a JSON array of nodes")
			Expect(outputSlice).NotTo(BeEmpty(), "Expected at least one node in MC")

			firstNode, ok := outputSlice[0].(map[string]interface{})
			Expect(ok).To(BeTrue(), "Each node entry should be a JSON object")
			Expect(firstNode).To(HaveKey("name"), "Node should have a name field")
			Expect(firstNode).To(HaveKey("status"), "Node should have a status field")
			GinkgoWriter.Printf("First node: name=%v status=%v\n", firstNode["name"], firstNode["status"])

			Expect(finalExec.Logs).NotTo(BeEmpty(),
				"Logs should be retrieved from S3 (execution.log)")
			GinkgoWriter.Printf("Execution %s: status=%s output_status=%s duration=%ds nodes=%d logs_len=%d\n",
				execution.ID, finalExec.Status, finalExec.OutputStatus,
				finalExec.Duration, len(outputSlice), len(finalExec.Logs))
		})

		It("should accept dry_run on write action and execute dry_run_action (no-wait)", func() {
			By("Fetching rollout_restart describe to get dry_run_action dynamically")
			descResp, err := apiClient.Get("/api/v0/trusted-actions/rollout_restart", accountID)
			Expect(err).NotTo(HaveOccurred())
			Expect(descResp.StatusCode).To(Equal(http.StatusOK))

			var desc struct {
				DryRunAction string `json:"dry_run_action"`
			}
			Expect(json.Unmarshal(descResp.Body, &desc)).To(Succeed())
			Expect(desc.DryRunAction).NotTo(BeEmpty(), "rollout_restart should declare a dry_run_action")
			GinkgoWriter.Printf("rollout_restart dry_run_action=%s\n", desc.DryRunAction)

			By("Dispatching rollout_restart with dry_run=true")
			body := map[string]interface{}{
				"target_cluster": mcName,
				"jira":           "ROSAENG-9999",
				"dry_run":        true,
				"params": map[string]string{
					"namespace": "kube-system",
					"name":      "coredns",
				},
			}
			resp, err := apiClient.Post("/api/v0/trusted-actions/rollout_restart/run", body, accountID)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

			var execution struct {
				ID             string `json:"id"`
				Action         string `json:"action"`
				ExecutedAction string `json:"executed_action"`
				DryRun         bool   `json:"dry_run"`
				Status         string `json:"status"`
			}
			Expect(json.Unmarshal(resp.Body, &execution)).To(Succeed())
			Expect(execution.ID).NotTo(BeEmpty())
			Expect(execution.Action).To(Equal("rollout_restart"))
			Expect(execution.ExecutedAction).To(Equal(desc.DryRunAction))
			Expect(execution.DryRun).To(BeTrue())
			Expect(execution.Status).To(Equal("pending"))
			GinkgoWriter.Printf("Dry-run dispatched: id=%s action=%s executed=%s\n",
				execution.ID, execution.Action, execution.ExecutedAction)
		})

		It("should enforce write cooldown and bypass with force (no-wait)", func() {
			By("Dispatching rollout_restart (first call)")
			body := map[string]interface{}{
				"target_cluster": mcName,
				"jira":           "ROSAENG-9999",
				"params": map[string]string{
					"namespace": "kube-system",
					"name":      "coredns",
				},
			}
			resp, err := apiClient.Post("/api/v0/trusted-actions/rollout_restart/run", body, accountID)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
			GinkgoWriter.Printf("First rollout_restart dispatched: %d\n", resp.StatusCode)

			By("Dispatching rollout_restart again (should hit cooldown)")
			resp, err = apiClient.Post("/api/v0/trusted-actions/rollout_restart/run", body, accountID)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusTooManyRequests))

			var errResp struct {
				Code string `json:"code"`
			}
			Expect(json.Unmarshal(resp.Body, &errResp)).To(Succeed())
			Expect(errResp.Code).To(Equal("ZOA-CREATE-007"))
			GinkgoWriter.Printf("Second call correctly rejected: %s\n", errResp.Code)

			By("Dispatching with force=true (should bypass cooldown)")
			body["force"] = true
			resp, err = apiClient.Post("/api/v0/trusted-actions/rollout_restart/run", body, accountID)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

			var execution struct {
				ID    string `json:"id"`
				Force bool   `json:"force"`
			}
			Expect(json.Unmarshal(resp.Body, &execution)).To(Succeed())
			Expect(execution.ID).NotTo(BeEmpty())
			Expect(execution.Force).To(BeTrue())
			GinkgoWriter.Printf("Force bypass succeeded: id=%s\n", execution.ID)
		})

		It("should dispatch get_events with params without waiting (no-wait)", func() {
			body := map[string]interface{}{
				"target_cluster": mcName,
				"jira":           "ROSAENG-9999",
				"params": map[string]string{
					"namespace": "kube-system",
				},
			}
			resp, err := apiClient.Post("/api/v0/trusted-actions/get_events/run", body, accountID)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

			var execution struct {
				ID     string            `json:"id"`
				Action string            `json:"action"`
				Params map[string]string `json:"params"`
				Status string            `json:"status"`
			}
			Expect(json.Unmarshal(resp.Body, &execution)).To(Succeed())
			Expect(execution.ID).NotTo(BeEmpty())
			Expect(execution.Action).To(Equal("get_events"))
			Expect(execution.Params).To(HaveKeyWithValue("namespace", "kube-system"))
			Expect(execution.Status).To(Equal("pending"))
			GinkgoWriter.Printf("Dispatched with params: id=%s action=%s ns=%s\n",
				execution.ID, execution.Action, execution.Params["namespace"])
		})
	})

	Describe("Audit", func() {
		It("should return audit log entries", func() {
			resp, err := apiClient.Get("/api/v0/trusted-actions/audit?limit=10", accountID)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var auditResp struct {
				Kind  string        `json:"kind"`
				Items []interface{} `json:"items"`
				Total int           `json:"total"`
			}
			Expect(json.Unmarshal(resp.Body, &auditResp)).To(Succeed())
			Expect(auditResp.Kind).To(Equal("AuditList"))
			Expect(auditResp.Items).NotTo(BeNil())
			GinkgoWriter.Printf("Audit log contains %d entries\n", auditResp.Total)
		})
	})
})
