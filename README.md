### res-log

Is a simple REST resource log. That listens for G Adventures REST API webhooks, and then fetches and stores public resouces anytime a hook is received.

For more info about the G Adventures REST API visit [G Adventures developer website](http://developers.gadventures.com).

# deploy notes
gcloud --project res-log app deploy
