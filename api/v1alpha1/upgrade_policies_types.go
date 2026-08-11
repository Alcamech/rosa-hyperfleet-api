/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ControlPlaneUpgradeType identifies the source of a control plane upgrade request.
// +kubebuilder:validation:Enum=UserInitiated;ServiceInitiated
type ControlPlaneUpgradeType string

// ScheduleUpgradeType indicates the type of schedule for the control plane upgrade.
// +kubebuilder:validation:Enum=Manual;Automatic
type ScheduleUpgradeType string

// UpgradeScopeType indicates if an automatic upgrade support only patch upgrades or patch and minor upgrades.
// +kubebuilder:validation:Enum=PatchOnly;PatchAndMinor
type UpgradeScopeType string

const (
	// UserInitiated represents a control plane upgrade policy defined by the user on a cluster.
	UserInitiated ControlPlaneUpgradeType = "UserInitiated"

	// ServiceInitiated represents a control plane upgrade policy defined by Red Hat.
	ServiceInitiated ControlPlaneUpgradeType = "ServiceInitiated"

	// ManualSchedule represents a control plane upgrade policy that happens one time for a specific version in a
	// specific time defined by the user.
	ManualSchedule ScheduleUpgradeType = "Manual"

	// AutomaticSchedule represents a recurrent control plane upgrade policy to the latest available upgrade.
	AutomaticSchedule ScheduleUpgradeType = "Automatic"

	// PatchOnly represents and automatic upgrade that only support patches.
	PatchOnly UpgradeScopeType = "PatchOnly"

	// PatchAndMinor represents and automatic upgrade that supports patches and minor versions
	PatchAndMinor UpgradeScopeType = "PatchAndMinor"
)

// Conditions
const (
	// ControlPlaneUpgradeStateConditionType is a Cluster condition that represents the state of a control plane
	// upgrade policy.
	ControlPlaneUpgradeStateConditionType = "ControlPlaneUpgradeState"
)

// Reasons
const (
	// UpgradePolicyStatePending reason indicates that the upgrade policy is pending scheduling.
	UpgradePolicyStatePending = "Pending"

	// UpgradePolicyStateScheduled reason indicates that the upgrade policy is scheduled.
	UpgradePolicyStateScheduled = "Scheduled"

	// UpgradePolicyStateStarted reason indicates that the upgrade policy has started.
	UpgradePolicyStateStarted = "Started"

	// UpgradePolicyStateCompleted reason indicates that the upgrade policy has successfully upgraded
	// to the target version.
	UpgradePolicyStateCompleted = "Completed"

	// UpgradePolicyStateFailed reason indicates that the upgrade policy hasn't successfully upgraded.
	UpgradePolicyStateFailed = "Failed"

	// UpgradePolicyStateCancelled reason indicates that the upgrade policy has been deleted by the user or Red Hat.
	UpgradePolicyStateCancelled = "Cancelled"
)

// ControlPlaneUpgradePolicySpec defines the desired control plane upgrade policy of a Cluster.
// +kubebuilder:validation:XValidation:rule="self.scheduleType != 'Manual' || (has(self.version) && has(self.nextRun))",message="version and nextRun are required when scheduleType is Manual"
// +kubebuilder:validation:XValidation:rule="self.scheduleType != 'Manual' || !has(self.schedule)",message="schedule must not be set when scheduleType is Manual"
// +kubebuilder:validation:XValidation:rule="self.scheduleType != 'Automatic' || has(self.schedule)",message="schedule is required when scheduleType is Automatic"
// +kubebuilder:validation:XValidation:rule="self.scheduleType != 'Automatic' || (!has(self.version) && !has(self.nextRun))",message="version and nextRun must not be set when scheduleType is Automatic"
// +kubebuilder:validation:XValidation:rule="self.scheduleType != 'Manual' || !has(self.upgradeScope)",message="upgradeScope must not be set when scheduleType is Manual"
// +kubebuilder:validation:XValidation:rule="self.scheduleType != 'Automatic' || has(self.upgradeScope)",message="upgradeScope is required when scheduleType is Automatic"
type ControlPlaneUpgradePolicySpec struct {

	// UpdateType indicates if it is a control plane upgrade policy defined by the user or
	// triggered by Red Hat for addressing critical CVEs.
	// +kubebuilder:validation:Required
	UpdateType ControlPlaneUpgradeType `json:"updateType"`

	// ScheduleType indicates if the control plane upgrade policy is "manual" and it's executed only one time or
	// whether it is "automatic" where an expression will calculate recurrent upgrades.
	// +kubebuilder:validation:Required
	ScheduleType ScheduleUpgradeType `json:"scheduleType"`

	// Schedule defines a cron expression that calculates the next automatic upgrade scheduling.
	// The cron expression must follow the standard 5-field format:
	// ┌───────────── minute (0 - 59)
	// │ ┌───────────── hour (0 - 23)
	// │ │ ┌───────────── day of month (1 - 31)
	// │ │ │ ┌───────────── month (1 - 12)
	// │ │ │ │ ┌───────────── day of week (0 - 6) (Sunday to Saturday)
	// │ │ │ │ │
	// * * * * *
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:Pattern=`^(\*|([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])|([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])-([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])|\*/([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])|([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])(,([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9]))*) (\*|([0-9]|1[0-9]|2[0-3])|([0-9]|1[0-9]|2[0-3])-([0-9]|1[0-9]|2[0-3])|\*/([0-9]|1[0-9]|2[0-3])|([0-9]|1[0-9]|2[0-3])(,([0-9]|1[0-9]|2[0-3]))*) (\*|([1-9]|1[0-9]|2[0-9]|3[0-1])|([1-9]|1[0-9]|2[0-9]|3[0-1])-([1-9]|1[0-9]|2[0-9]|3[0-1])|\*/([1-9]|1[0-9]|2[0-9]|3[0-1])|([1-9]|1[0-9]|2[0-9]|3[0-1])(,([1-9]|1[0-9]|2[0-9]|3[0-1]))*) (\*|([1-9]|1[0-2])|([1-9]|1[0-2])-([1-9]|1[0-2])|\*/([1-9]|1[0-2])|([1-9]|1[0-2])(,([1-9]|1[0-2]))*) (\*|[0-6]|[0-6]-[0-6]|\*/[0-6]|[0-6](,[0-6])*)$`
	// +optional
	Schedule *string `json:"schedule,omitempty"`

	// Version is the desired upgrade version on "manual" upgrade policies.
	// +kubebuilder:validation:MaxLength=64
	// +optional
	Version *string `json:"version,omitempty"`

	// NextRun is the time the upgrade should run for "manual" upgrade policies
	// +optional
	NextRun *metav1.Time `json:"nextRun,omitempty"`

	// UpgradeScope indicates if minor version upgrades are allowed for automatic upgrades.
	// Manual upgrades always allow it.
	// +optional
	UpgradeScope UpgradeScopeType `json:"upgradeScope,omitempty"`
}

type ControlPlaneUpgradePolicyStatus struct {
	// NextRun is the time when the control plane will upgrade.
	// When the ScheduleType is "manual" it will match with the NextRun defined by the user.
	// When the ScheduleType is "automatic" it will be calculated from the Schedule cron expression.
	NextRun *metav1.Time `json:"nextRun,omitempty"`
}
