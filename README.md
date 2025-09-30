### WCFC-Updater

This app performs scheduled updates for the WCFC applications in Google Cloud.  It works as follows:

* Retrieve /api/version to find out what commit the app is currently running from.
* Look in the git history to see if the app needs to be auto-updated.  This is true if:
  * There are Dependabot commits after the running commit, and
  * The current head commit on the main branch is not tagged, and
  * There are _not_ any non-Dependabot (i.e. human-written) commits after the running commit.  (Humans can deploy their own code in their own good time.)
* If the app needs to be auto-updated, then use GitHub App authentication to launch the repo's build-and-release action via workflow dispatch.

