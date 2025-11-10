# Secrets Bootstrap Module

Renders a SealedSecret manifest describing baseline secrets required for each
environment. Actual encryption keys are handled by the Sealed Secrets
controller; this module only produces the skeleton manifest for commit into GitOps automation.
