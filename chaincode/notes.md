A) Site Lifecycle
   - RegisterSite
   - UpdateSiteStatus (replaces Suspend/Revoke)
   - AttestSiteInfrastructure (renamed from AttestSite)
   - GetSite

B) Policy Lifecycle  
   - RegisterPolicy
   - UpdatePolicyStatus (replaces Activate/Deprecate)
   - GetPolicy

C) Federation Job Lifecycle
   - CreateFederationJob
   - RegisterJobParticipants
   - GetJob

D) Training Authorization
   - GrantTrainingAccess
   - RenewTrainingAccess
   - RevokeTrainingAccess
   - VerifyTrainingEligibility
   - GetGrant / ListGrantsForJob

E) Round + Provenance (Audit Spine)
   - StartRound
   - SubmitSiteUpdate
   - CommitRound
   - AnchorModelArtifact
   - GetRound / ListUpdatesForRound / ListArtifactsForJob

F) CODE ATTESTATION (NEW - replaces your custom auth)
   - SubmitCodePackage
   - UpdateCodeStatus (consolidated approve/reject/revoke)
   - VerifyCodeForExecution
   - RegisterCodePolicy
   - GetCodePackage / ListCodePackagesForSite / FindCodePackageByHash

G) Minimal NVFlare Audit
   - StoreJobSubmission
   - LogClientShutdown



They work together:

1. Site registers → RegisterSite
2. Site proves secure hardware → AttestSiteInfrastructure (TPM quote)
3. Site submits training code → SubmitCodePackage (static analysis hash)
4. Platform approves code → UpdateCodeStatus → APPROVED
5. NVFlare starts job → VerifyTrainingEligibility (checks site status + grant)
6. NVFlare runs code → VerifyCodeForExecution (checks code hash matches on-chain)