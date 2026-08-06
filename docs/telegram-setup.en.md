# Telegram Setup for IncidentGPT

This guide takes you from an empty Telegram setup to a working IncidentGPT configuration. You will create one Telegram bot, one Telegram channel, and one linked discussion group.

Flow:

```text
Alertmanager
    |
    v
IncidentGPT
    |
    v
Telegram bot
    |-- publishes incidents to a Telegram channel
    `-- works with the linked discussion group
```

| Entity | Purpose |
|--------|---------|
| Telegram bot | A technical account that IncidentGPT uses to send messages |
| Telegram channel | The publication feed for incidents |
| Telegram group | The chat for discussions and comments under channel posts |

> [!IMPORTANT]
> A Telegram group is already a chat. Do not create a separate "group" and a separate "chat". You need one bot, one channel, and one discussion group.

## How IncidentGPT Uses Telegram

The current `ai-worker` code sends messages through the Telegram Bot API `sendMessage` method.

What `ai-worker` does:

1. Publishes a raw alert to `TELEGRAM_CHANNEL_ID`.
2. Publishes a grouped-alert summary to `TELEGRAM_CHANNEL_ID`.
3. Publishes the AI analysis as a reply to the channel message by using `reply_to_message_id`.
4. Does not send the `message_thread_id` parameter.
5. Does not read regular group messages.
6. Does not call `getUpdates` during normal operation.

`TELEGRAM_THREAD_CHAT_ID` in the current configuration means the ID of the linked discussion group. The code loads this variable, shows it in `/status` as `thread_chat_configured`, and uses `TELEGRAM_CHANNEL_ID` as a fallback when it is empty. The current send path uses `TELEGRAM_CHANNEL_ID`; comments appear in the linked discussion group because of the Telegram channel discussion setting.

> [!NOTE]
> `TELEGRAM_THREAD_CHAT_ID` is not a forum topic ID. The current code has no separate setting for `message_thread_id`.

Current message formats:

| Message type | Where it is published | Format |
|--------------|-----------------------|--------|
| Raw alert | `TELEGRAM_CHANNEL_ID` | `[STATUS] alertname (severity)`, then `Labels`, `Annotations`, `Starts`, and `Source` |
| Group summary | `TELEGRAM_CHANNEL_ID` | `[STATUS] N linked alerts (group_key)`, then the alert list |
| AI analysis | Reply to the channel message | LLM structure: root cause, fix, prevention; long text is split into chunks up to 4000 characters |

## What You Need

- [ ] A Telegram account.
- [ ] Access to the official `@BotFather` account.
- [ ] Administrator rights in the channel and group you create.
- [ ] Access to the IncidentGPT configuration.
- [ ] Telegram bot token.
- [ ] Telegram channel ID.
- [ ] Telegram group ID.
- [ ] OpenRouter API key for `ai-worker`.
- [ ] Installed `kubectl` and Helm for deployment.

> [!WARNING]
> Do not publish the bot token, commit it to Git, include it in screenshots, or send it to other people. The secrets file must be listed in `.gitignore`. If the token is exposed, rotate it through BotFather.

## Step 1. Create a Telegram Bot with BotFather

1. Open Telegram.
2. Search for `@BotFather`.
3. Open the official account.
4. Check that the username is `BotFather`.
5. Click `Start`.
6. Send this command:

```text
/newbot
```

7. When BotFather asks for the bot display name, enter:

```text
IncidentGPT Orange
```

8. When BotFather asks for the username, enter a unique name that ends with `bot`:

```text
orange_incident_bot
```

9. Copy the token that BotFather sends back.

Example token format:

```text
1234567890:AAExampleTokenDoNotUse
```

> [!WARNING]
> This is only an example. Do not use it as a real token.

In the IncidentGPT configuration this token maps to:

```bash
TELEGRAM_BOT_TOKEN="<TOKEN_FROM_BOTFATHER>"
```

### How to Know It Worked

- BotFather says the bot was created.
- BotFather sends a link to the bot.
- You receive a token.
- You can open a private chat with the bot.
- The `/start` command sends without an error.

### If the Username Is Already Taken

1. Do not change the token you already received.
2. Choose another username.
3. Make sure the username ends with `bot`.
4. Try one of these examples:

```text
incidentgpt_orange_bot
orange_incident_bot
my_incidentgpt_bot
```

The username must be unique across Telegram.

## Step 2. Configure Privacy Mode

For normal IncidentGPT operation, you do not need to disable Privacy Mode.

Reason: the current `ai-worker` does not read regular group messages. It only sends messages through `sendMessage`. Commands such as `/test@username_bot` are delivered to the bot even when Privacy Mode is enabled, and that is enough to get the group ID manually through `getUpdates`.

Disable Privacy Mode only if you modify the project so the bot must read regular group messages.

If you still need to disable Privacy Mode:

1. Open `@BotFather`.
2. Send this command:

```text
/setprivacy
```

3. Select your bot.
4. Select `Disable`.
5. Wait for the confirmation from BotFather.

## Step 3. Create the Discussion Group

A group is a chat. You do not need to create another chat after creating the group.

Example names:

```text
IncidentGPT Discussion
```

```text
inc orange chat
```

### Telegram Desktop

1. Open Telegram Desktop.
2. Click the button for creating a new chat.
3. Select `New Group`.
4. Select at least one temporary member if Telegram requires it.
5. Enter the group name.
6. Click `Create`.
7. Open the group settings.
8. If the group should be closed, keep it private.

### Telegram Mobile

1. Open Telegram on your phone.
2. Tap the button for creating a new message.
3. Select `New Group`.
4. Select a member if Telegram requires this step.
5. Enter the group name.
6. Tap `Create`.
7. Open the group profile.
8. Check that it is a group, not a channel.

Forum topics are not required for the current implementation because the code does not send `message_thread_id`.

If you want topics for manual organization:

1. Open the group.
2. Open group management.
3. Open `Topics`.
4. Click `Enable`.
5. Check that the `General` topic appears.

> [!IMPORTANT]
> Do not use the `General` topic ID instead of the group ID. IncidentGPT currently expects the group chat id in `TELEGRAM_THREAD_CHAT_ID`, not a topic ID.

## Step 4. Add the Bot to the Group

1. Open the group.
2. Open group management.
3. Select `Administrators`.
4. Click `Add Administrator`.
5. Find the bot by username, for example `orange_incident_bot`.
6. Add the bot.
7. Grant the minimum required rights.

| Right | Required | Why |
|-------|----------|-----|
| Send messages | Yes | Needed for manual `sendMessage` checks in the group and future discussion flows |
| Manage topics | No | The current code does not send `message_thread_id` and does not manage topics |
| Delete messages | No | Not required for normal operation |
| Add users | No | Not required |
| Pin messages | No | The current code does not pin messages |

### How to Check

- The bot appears in the group member list.
- The bot has an administrator mark.
- The bot is allowed to send messages.

## Step 5. Create the Telegram Channel

A channel is a publication feed. A group is for discussions. A bot is the sender.

Example names:

```text
IncidentGPT Alerts
```

```text
inc orange
```

A private channel is fine.

### Create a Private Channel

1. Open Telegram.
2. Click the button for creating a new chat or message.
3. Select `New Channel`.
4. Enter the channel name.
5. Click `Create`.
6. Select `Private Channel`.
7. Save the channel.

### How to Check That It Is a Channel

- The top area shows subscribers, not members.
- The input field is called `Post`, `Publication`, or a similar label.
- It is not a forum.
- There is no `General` topic in the channel.

### How Not to Mix Up a Channel and a Group

| Sign | Channel | Group |
|------|---------|-------|
| People are called | Subscribers | Members |
| Main purpose | Publications | Conversation |
| General topic | No | Possible |
| Bot rights | Publish messages | Send messages |

## Step 6. Link the Group to the Channel

1. Open the channel.
2. Open channel management.
3. Select `Discussion`.
4. Select the group you created.
5. Confirm the link.

Result:

- new channel posts will appear in the linked group;
- people can discuss posts under the channel publications;
- you do not need to delete the group or create another chat.

### How to Check

1. Open the channel settings.
2. Find the `Discussion` section.
3. Check that it shows your group name.
4. Publish a test message to the channel.
5. Open the group.
6. Check that the test publication appears in the discussion group.

Telegram may send a service message about the link. That is normal.

## Step 7. Add the Bot as a Channel Administrator

1. Open the channel.
2. Open channel management.
3. Select `Administrators`.
4. Click `Add Administrator`.
5. Find your bot by username.
6. Add the bot.
7. Enable `Post Messages`.
8. Save the changes.

| Channel right | Needed by the current code |
|---------------|----------------------------|
| Post messages | Yes |
| Edit messages | No |
| Delete messages | No |
| Manage stories | No |
| Add subscribers | No |

### How to Check

- The bot is in the channel administrator list.
- The bot has the right to publish posts.
- A new post can be sent through Telegram Bot API `sendMessage`.

## Step 8. Get the Group ID

> [!WARNING]
> Never send a URL with the token to other people. Never publish it in issues, documentation, or screenshots.

1. Open the discussion group.
2. In the `General` topic or in the normal group chat, send:

```text
/test@BOT_USERNAME
```

Example:

```text
/test@orange_incident_bot
```

3. Open this URL in a browser. Replace `<TELEGRAM_BOT_TOKEN>` with the real token:

```text
https://api.telegram.org/bot<TELEGRAM_BOT_TOKEN>/getUpdates
```

4. Find an object where `message.chat.type` is `supergroup`.
5. Copy `message.chat.id` completely, including the minus sign.

An anonymized example:

```json
{
  "ok": true,
  "result": [
    {
      "update_id": 111111111,
      "message": {
        "message_id": 25,
        "from": {
          "id": 222222222,
          "is_bot": false,
          "first_name": "Example"
        },
        "chat": {
          "id": -1001234567890,
          "title": "IncidentGPT Discussion",
          "type": "supergroup"
        },
        "text": "/test@orange_incident_bot"
      }
    }
  ]
}
```

Result:

```bash
TELEGRAM_THREAD_CHAT_ID="-1001234567890"
```

Important details:

- `type: "supergroup"` means this is the group.
- The ID usually starts with `-100`.
- Do not remove the minus sign.
- `message_id` is not the group ID.
- `update_id` is not the group ID.
- `from.id` is the user ID, not the group ID.
- `message_thread_id` is the topic ID, not the group ID.

## Step 9. Get the Channel ID

1. Make sure the bot is already a channel administrator.
2. Publish a new test post in the channel:

```text
test channel
```

3. Open again:

```text
https://api.telegram.org/bot<TELEGRAM_BOT_TOKEN>/getUpdates
```

4. Find the `channel_post` object.
5. Copy `channel_post.chat.id` completely, including the minus sign.

An anonymized example:

```json
{
  "ok": true,
  "result": [
    {
      "update_id": 333333333,
      "channel_post": {
        "message_id": 7,
        "chat": {
          "id": -1009876543210,
          "title": "IncidentGPT Alerts",
          "type": "channel"
        },
        "text": "test channel"
      }
    }
  ]
}
```

Result:

```bash
TELEGRAM_CHANNEL_ID="-1009876543210"
```

Important details:

- `type: "channel"` confirms that this is the channel.
- `sender_chat.id` and `chat.id` usually match.
- Use `chat.id`.
- Keep the minus sign.

## What to Do If getUpdates Returns result: []

The response may look like this:

```json
{
  "ok": true,
  "result": []
}
```

This is not always an error. It means the bot currently has no new updates to show.

Check in this order:

1. Make sure the bot is added to the group.
2. Send a new command after opening `getUpdates`:

```text
/test@BOT_USERNAME
```

3. Refresh the `getUpdates` page.
4. For the channel, create a new post after adding the bot as an administrator.
5. Refresh `getUpdates` again.

Check the webhook:

```text
https://api.telegram.org/bot<TELEGRAM_BOT_TOKEN>/getWebhookInfo
```

If the `url` field is not empty, the bot has a webhook configured. Telegram does not use `getUpdates` and webhook delivery at the same time.

Delete the webhook without clearing pending updates:

```text
https://api.telegram.org/bot<TELEGRAM_BOT_TOKEN>/deleteWebhook?drop_pending_updates=false
```

After deleting the webhook:

1. Send a new message to the group or a new post to the channel.
2. Open `getUpdates` again.

Other reasons for an empty result:

- the command was sent before the bot was added;
- the message was sent to the wrong group;
- the bot username was typed incorrectly;
- the bot was removed from the group;
- the bot is not a channel administrator;
- the channel message was published before the bot was added;
- you are using another bot's token;
- a running `ai-worker` instance or another process already consumed the updates;
- the application uses a webhook;
- updates were cleared with `drop_pending_updates=true`.

> [!NOTE]
> The current IncidentGPT `ai-worker` does not call `getUpdates`. If updates disappear, another process or manual check may have consumed them.

## If the Token Was Accidentally Committed or Shown in a Screenshot

1. Open `@BotFather`.
2. Send or select `/mybots`.
3. Select the bot.
4. Open `API Token`.
5. Revoke the old token.
6. Create a new token.
7. Update the Kubernetes Secret or `values-secret.yaml`.
8. Restart `ai-worker`.
9. Remove the token from Git history if it was committed.

> [!IMPORTANT]
> Removing the line from the latest commit is not enough. The old token remains in Git history until the history is cleaned. Even after cleaning the history, treat the old token as compromised and replace it.

## Step 10. Fill in the IncidentGPT Telegram Configuration

The umbrella chart uses `deploy/incidentgpt/values.yaml`. For secrets, use a separate local `values-secret.yaml` file. This file is already listed in `.gitignore`.

Do not copy the demo IDs or tokens into production.

Minimal `values-secret.yaml` example:

```yaml
ai-worker:
  secretValues:
    openrouterApiKey: "<OPENROUTER_API_KEY>"
    telegramBotToken: "<TOKEN_FROM_BOTFATHER>"
    sanitizerAuthSharedSecret: "<SAME_HMAC_SECRET_AS_SANITIZER>"

  env:
    TELEGRAM_CHANNEL_ID: "-1009876543210"
    TELEGRAM_THREAD_CHAT_ID: "-1001234567890"

incidentgpt-enricher:
  secretValues:
    sanitizerAuthSharedSecret: "<SAME_HMAC_SECRET_AS_SANITIZER>"

incidentgpt-sanitizer:
  secretValues:
    hashKey: "<LONG_RANDOM_HASH_KEY>"
    authSharedSecret: "<SAME_HMAC_SECRET_AS_SANITIZER>"
```

If you use an existing Kubernetes Secret, the configuration can look like this:

```yaml
ai-worker:
  secrets:
    existingSecret: "incidentgpt-ai-worker"
    openRouterApiKeyKey: "OPENROUTER_API_KEY"
    telegramBotTokenKey: "TELEGRAM_BOT_TOKEN"
    sanitizerAuthSharedSecretKey: "auth-shared-secret"

  env:
    TELEGRAM_CHANNEL_ID: "-1009876543210"
    TELEGRAM_THREAD_CHAT_ID: "-1001234567890"
```

Recommended `.gitignore` entries:

```gitignore
values-secret.yaml
*.secret.yaml
.env
.env.local
```

In the current repository, `values-secret.yaml`, `.env`, and `.env.*` are already ignored. If you use a name such as `prod.secret.yaml`, add the `*.secret.yaml` rule.

## Step 11. Test the Bot Before Starting IncidentGPT

Set a variable in the current shell. Do not paste the token into shell history on shared machines.

```bash
export TELEGRAM_BOT_TOKEN='<TOKEN_FROM_BOTFATHER>'
```

Test the token:

```bash
curl --request GET \
  "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/getMe"
```

Expected response:

```json
{
  "ok": true,
  "result": {
    "is_bot": true,
    "username": "orange_incident_bot"
  }
}
```

Test publishing to the channel:

```bash
curl --request POST \
  "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage" \
  --header "Content-Type: application/json" \
  --data '{
    "chat_id": "-1009876543210",
    "text": "IncidentGPT Telegram test"
  }'
```

Test sending to the group:

```bash
curl --request POST \
  "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage" \
  --header "Content-Type: application/json" \
  --data '{
    "chat_id": "-1001234567890",
    "text": "IncidentGPT discussion test"
  }'
```

Expected result for both checks: JSON with `"ok": true`.

> [!NOTE]
> The current IncidentGPT implementation does not use `message_thread_id`, so a production example with `message_thread_id` is not needed.

## Step 12. Apply the Settings and Check ai-worker

Build umbrella chart dependencies:

```bash
helm dependency build deploy/incidentgpt
```

Install or upgrade the release:

```bash
helm upgrade --install incidentgpt ./deploy/incidentgpt \
  --namespace incidentgpt \
  --create-namespace \
  -f deploy/incidentgpt/values.yaml \
  -f values-secret.yaml
```

Check the Pod:

```bash
kubectl get pods -n incidentgpt
```

Find the Deployment name:

```bash
kubectl get deployments -n incidentgpt
```

Read recent `ai-worker` logs:

```bash
kubectl logs -n incidentgpt deployment/ai-worker --tail=200
```

Follow logs in real time:

```bash
kubectl logs -n incidentgpt deployment/ai-worker --follow
```

If the Deployment name is different, use the name from `kubectl get deployments -n incidentgpt`.

The logs should show configured `channel_id`, `thread_chat_id`, and `parse_mode`. There should be no `telegram_error` messages.

## Final Checklist

- [ ] The bot was created with BotFather.
- [ ] The token is stored in a Secret.
- [ ] The Telegram channel was created.
- [ ] The Telegram group was created.
- [ ] The group is linked to the channel.
- [ ] The bot is a group administrator.
- [ ] The bot is a channel administrator.
- [ ] The bot can publish messages to the channel.
- [ ] The group ID was obtained from `type=supergroup`.
- [ ] The channel ID was obtained from `type=channel`.
- [ ] IDs were saved with the minus sign.
- [ ] `values-secret.yaml` is listed in `.gitignore`.
- [ ] `getMe` returns `ok=true`.
- [ ] `sendMessage` to the channel works.
- [ ] `sendMessage` to the group works.
- [ ] `ai-worker` is running.
- [ ] `ai-worker` logs have no Telegram errors.

## Quick Check Table

| What to check | Expected value |
|---------------|----------------|
| Group type | `supergroup` |
| Channel type | `channel` |
| Group ID | negative number, usually starts with `-100` |
| Channel ID | negative number, usually starts with `-100` |
| `TELEGRAM_CHANNEL_ID` | channel ID |
| `TELEGRAM_THREAD_CHAT_ID` | discussion group ID; the current code loads it but sends messages to `TELEGRAM_CHANNEL_ID` |
| Bot in group | administrator with the right to send messages |
| Bot in channel | administrator with the right to publish messages |
| `getMe` | `ok=true` |
| `sendMessage` | returns `ok=true` |

## Common Errors

### `result: []`

Symptom: `getUpdates` returns an empty array.

Cause: the bot has no new updates, a webhook is consuming updates, the message was sent before the bot was added, or another process already read the updates.

Fix: send a new `/test@BOT_USERNAME` command, refresh `getUpdates`, check `getWebhookInfo`, and run `deleteWebhook?drop_pending_updates=false` if needed.

### `Unauthorized`

Symptom: Telegram API returns `Unauthorized`.

Cause: wrong token, extra space, newline, or revoked token.

Fix: copy a new token from BotFather, update the Secret or `values-secret.yaml`, and restart `ai-worker`.

### `Bad Request: chat not found`

Symptom: `sendMessage` cannot find the chat.

Cause: the bot was not added to the channel or group, the ID is wrong, the minus sign was removed, or an invite link was used instead of an ID.

Fix: add the bot, get `chat.id` again, and store the ID as a string:

```yaml
TELEGRAM_CHANNEL_ID: "-1009876543210"
```

### `Forbidden: bot is not a member of the channel chat`

Symptom: the bot cannot send a message to the channel.

Cause: the bot is not a channel administrator.

Fix: open the channel, add the bot as an administrator, and enable the right to publish posts.

### `Forbidden: bot was kicked from the supergroup chat`

Symptom: the bot cannot send a message to the group.

Cause: the bot was removed from the group or blocked.

Fix: add the bot back and grant the right to send messages.

### `Not enough rights to send text messages to the chat`

Symptom: Telegram does not allow sending text.

Cause: the bot cannot send messages in the group or cannot publish in the channel.

Fix: check administrator rights in the target chat.

### `Bad Request: message thread not found`

Symptom: Telegram says the thread was not found.

Cause: an incorrect `message_thread_id` is being used somewhere.

Fix: for the current IncidentGPT code, do not configure `message_thread_id`; the code does not send it. If your fork has that setting, get the topic ID from Telegram API JSON and do not confuse it with `chat.id`.

### The Bot Sends to the Group but Not to the Channel

Symptom: manual `sendMessage` to the group works, but sending to the channel fails.

Cause: the bot is not a channel administrator or cannot publish posts.

Fix: add the bot as a channel administrator and enable `Post Messages`.

### The Bot Sends to the Channel but the Message Does Not Appear in the Discussion

Symptom: the post exists in the channel, but not in the group.

Cause: the channel and group are not linked through `Discussion`.

Fix: open channel management, select `Discussion`, link the group, and publish a new test post.

### `TELEGRAM_CHANNEL_ID` and `TELEGRAM_THREAD_CHAT_ID` Are Swapped

Symptom: alerts go to the wrong place or Telegram returns an error.

Cause: the channel ID and group ID were pasted into the wrong variables.

Fix: paste `type=channel` into `TELEGRAM_CHANNEL_ID`, and paste `type=supergroup` into `TELEGRAM_THREAD_CHAT_ID`.

### The Minus Sign Was Removed from the ID

Symptom: `chat not found` or the message goes to the wrong place.

Cause: `1009876543210` was copied instead of `-1009876543210`.

Fix: save the full ID, including `-`.

### `update_id` Was Copied Instead of `chat.id`

Symptom: Telegram cannot find the chat.

Cause: `update_id` is the event number, not the channel or group ID.

Fix: use only `message.chat.id` or `channel_post.chat.id`.

### `from.id` Was Copied Instead of `chat.id`

Symptom: the bot sends to a user or returns an error.

Cause: `from.id` is the user ID.

Fix: copy `chat.id`.

### An Invite Link Is Used Instead of a Numeric ID

Symptom: values contain `https://t.me/...` or `t.me/+...`.

Cause: Telegram Bot API `sendMessage` expects `chat_id`, not an invite link.

Fix: get the numeric `chat.id` through `getUpdates`.

### The Token Contains a Space or Newline

Symptom: `Unauthorized`, or the variable looks configured but Telegram rejects requests.

Cause: the token was copied with an extra character.

Fix: paste the token again without spaces or newlines.

### YAML Indentation Is Broken

Symptom: Helm does not apply the values or the Pod starts without variables.

Cause: `ai-worker`, `secretValues`, and `env` are at the wrong level.

Fix: check the structure:

```yaml
ai-worker:
  secretValues:
    telegramBotToken: "<TOKEN_FROM_BOTFATHER>"
  env:
    TELEGRAM_CHANNEL_ID: "-1009876543210"
```

### YAML Interprets the Numeric ID Incorrectly

Symptom: the value behaves like a number, not a string.

Cause: the ID was written without quotes.

Fix: store Telegram IDs as strings:

```yaml
TELEGRAM_CHANNEL_ID: "-1009876543210"
```

not:

```yaml
TELEGRAM_CHANNEL_ID: -1009876543210
```

### The Bot Receives Updates Through a Webhook, Not `getUpdates`

Symptom: `getUpdates` is empty even though messages were sent.

Cause: webhook delivery is enabled for the bot.

Fix: check `getWebhookInfo` and delete the webhook with `deleteWebhook?drop_pending_updates=false`.

### A Running ai-worker Consumes Updates Before the Browser

Symptom: updates disappear.

Cause: another process calls `getUpdates`.

Fix: stop that process while getting IDs. The current IncidentGPT `ai-worker` does not call `getUpdates`.

### Privacy Mode Prevents Reading Regular Messages

Symptom: the bot does not see regular text in the group.

Cause: Privacy Mode is enabled.

Fix: for getting IDs, send `/test@BOT_USERNAME`. For a fork that reads regular messages, disable Privacy Mode through `/setprivacy`.

### The Channel and Group Are Not Linked

Symptom: comments do not appear in the group.

Cause: the channel has no discussion group selected.

Fix: link the group in the `Discussion` section.

### Only a Group Was Created, No Channel

Symptom: there is no object with `type=channel`.

Cause: a chat was created for conversation, but no publication channel exists.

Fix: create `New Channel` and get its `chat.id`.

### Only a Channel Was Created, No Discussion Group

Symptom: alerts are published, but there is nowhere to discuss them.

Cause: the group was not created or not linked.

Fix: create a group and link it to the channel.

### Two Groups Were Created Instead of a Channel and a Group

Symptom: both objects in `getUpdates` have `type=supergroup`.

Cause: the channel was not created.

Fix: create `New Channel`, not another group.

### A Forum Topic Was Created and Treated as a Separate Chat

Symptom: `message_thread_id` is used instead of `chat.id`.

Cause: a forum topic is inside a group. It is not a separate chat.

Fix: use the group `chat.id` for `TELEGRAM_THREAD_CHAT_ID`. The current code does not configure a separate `message_thread_id`.

## Exactly What to Paste Where

| What you got | Where to paste it |
|--------------|-------------------|
| Token from BotFather | `TELEGRAM_BOT_TOKEN` or `ai-worker.secretValues.telegramBotToken` |
| `chat.id` of an object with `type=channel` | `TELEGRAM_CHANNEL_ID` |
| `chat.id` of an object with `type=supergroup` | `TELEGRAM_THREAD_CHAT_ID` |
| Forum topic ID | Only into a `message_thread_id` setting, if your fork has one |

Telegram returned:

```json
{
  "type": "channel",
  "id": -1009876543210
}
```

So use:

```bash
TELEGRAM_CHANNEL_ID="-1009876543210"
```

Telegram returned:

```json
{
  "type": "supergroup",
  "id": -1001234567890
}
```

So use:

```bash
TELEGRAM_THREAD_CHAT_ID="-1001234567890"
```
