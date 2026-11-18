# Integration guide

How to connect Wherehouse to Paperless-ngx, Warranteed and notification services. Installing Wherehouse itself is covered in the [README](../README.md), and using it in the [user guide](user-guide.md).

- [How the pieces fit](#how-the-pieces-fit)
- [Paperless-ngx](#paperless-ngx)
- [Warranteed](#warranteed)
- [Reminders](#reminders)
- [Your own scripts](#your-own-scripts)
- [Troubleshooting](#troubleshooting)

## How the pieces fit

| Service | What Wherehouse does with it | Set up by |
| --- | --- | --- |
| Paperless-ngx | Links manuals and receipts kept in Paperless to things, and links them back | The admin sets the URL; each person adds their own token |
| Warranteed | Keeps purchases and warranties in step with things | Each person connects from Warranteed |
| Notification services | Sends maintenance reminders | Each person, under **Settings → Reminders** |

None of them is needed; Wherehouse works on its own.

## Paperless-ngx

### Connecting

1. Set `PAPERLESS_URL` to where Wherehouse reaches Paperless, e.g. `http://paperless:8000` when both run in the same Docker network. If browsers use another address, set `PAPERLESS_PUBLIC_URL` to it, so document links open. Restart.
2. Each person pastes their own API token under **Settings → Paperless-ngx** (in Paperless: *Profile → API Auth Token*) and clicks **Test**. Paperless's permissions then decide which documents they can link. Giving each person their own Paperless user is recommended.

`PAPERLESS_TOKEN` is optional and only ever used for admins who haven't saved a token of their own; other people never get it.

On a thing, **Documents → From Paperless** searches your Paperless and links the documents you pick as a manual, receipt, warranty or other document. A linked document's name opens it in Paperless. Its download button gets the file for everyone in the household through the connection of whoever linked it, so others can get that document but nothing else in your Paperless. Removing it only unlinks it; the document stays in Paperless.

### Links back from Paperless

With **Link documents back to Wherehouse** on, under **Settings → Paperless-ngx**, documents linked to things get a link in a URL custom field in Paperless, so a receipt in Paperless leads to the thing it belongs to. The link opens the thing, or a list of the things when the document is on several, and when they are in another of your households, the page offers to switch to it. Turning the setting on also fills in the link on the documents you linked before, and says how many it updated.

The link follows the document: it changes when the document is linked to another thing or taken off one, and goes when the document is on no thing any more. Links written while Wherehouse had another address are updated too; a value you typed into the field yourself is left alone.

The field is called "Wherehouse" (rename it with `PAPERLESS_FIELD`) and is created on first use. Your token must be allowed to change documents, and to create custom fields if the field doesn't exist yet.

## Warranteed

[Warranteed](https://github.com/fhavrlent/warranteed) tracks purchases and warranties. Connected, a warranty and the thing it's for are linked and kept in step: Warranteed owns the purchase and warranty details, which Wherehouse shows read-only; Wherehouse owns where the thing is, its photos, label, insurance and sale. Claims become entries in the thing's maintenance log. The [Warranteed integration guide](https://github.com/fhavrlent/warranteed/blob/main/docs/integrations.md#wherehouse) describes every detail of what's kept in step.

### Connecting

1. In Warranteed, set `WHEREHOUSE_URL` to where Warranteed reaches Wherehouse, and `WHEREHOUSE_PUBLIC_URL` if browsers use another address.
2. If Warranteed runs on your local network, beside Wherehouse in Docker for example, set `WHEREHOUSE_HOOK_ALLOWED_HOSTS` in Wherehouse to its host name (e.g. `warranteed`). Restart both.
3. Each person clicks **Connect to Wherehouse** under **Settings → Wherehouse** in Warranteed. Wherehouse asks which households to connect and shows the address of the app asking; **Allow** sends them back connected. There's no token to copy.

Both apps in one Docker Compose file, in the same network:

```yaml
services:
  wherehouse:
    image: ghcr.io/fhavrlent/wherehouse:latest
    env_file: wherehouse.env   # WHEREHOUSE_HOOK_ALLOWED_HOSTS=warranteed
    volumes:
      - wherehouse-data:/data
  warranteed:
    image: ghcr.io/fhavrlent/warranteed:latest
    env_file: warranteed.env   # WHEREHOUSE_URL=http://wherehouse:8080
    volumes:
      - warranteed-data:/data

volumes:
  wherehouse-data:
  warranteed-data:
```

### What a connected app can do

Connecting creates a token under **Settings → API tokens** for each household connected, named after the app. Each acts as the person who allowed it, in its household, and can do only what keeping things in step needs: read and change things, their photos, documents, links and maintenance, and follow changes. It can't export the household, change its kinds, people or settings, or create other tokens. Deleting the token, or disconnecting in Warranteed, disconnects the app.

### Being told about changes

A connected app can ask Wherehouse to ping it whenever something in the household changes, so a thing moved, sold or archived here shows in Warranteed within seconds. The ping is signed and carries no data. For addresses that come from apps, Wherehouse connects only to public addresses and to the hosts in `WHEREHOUSE_HOOK_ALLOWED_HOSTS`, so a token can't be used to reach the rest of your network. Without a ping, Warranteed still finds the changes when it next looks, every 10 minutes.

## Reminders

Under **Settings → Reminders**, each person adds one or more [Shoutrrr URLs](https://shoutrrr.nickfedor.com/latest/services/overview/) and how many days before maintenance is due to be reminded (`7, 0` by default: a week before and on the day). **Send a test** checks them. Reminders go out once a day after `WHEREHOUSE_NOTIFY_HOUR` in the `TZ` timezone, each once.

```
ntfy://ntfy.sh/my-house
gotify://gotify.example.com/AbCdEf123456
discord://token@webhookid
telegram://token@telegram?chats=@my-channel
smtp://user:password@smtp.example.com:587/?from=house@example.com&to=me@example.com
```

Discord, Telegram, Slack, Pushover, Pushbullet, IFTTT and Join work for everyone. Services where the URL holds a server address (ntfy, Gotify, generic webhooks, email and others) are open to admins; other people can use them only with hosts listed in `WHEREHOUSE_NOTIFY_ALLOWED_HOSTS`, e.g. `ntfy.sh,ntfy.example.com`, so the server can't be used to reach internal addresses.

## Your own scripts

**Settings → API tokens → Create token** makes a token for your own scripts. It acts as you, in the household you were in when you made it, and can do everything you can except what needs a signed-in session (managing tokens and settings, restoring backups) and what only admins can. Send it as `Authorization: Bearer <token>`:

```sh
curl -H "Authorization: Bearer $TOKEN" "https://wherehouse.example.com/api/search?q=drill"
```

[api.md](api.md) describes the API, including how an app connects.

## Troubleshooting

**Paperless says it can't connect.** `PAPERLESS_URL` must be reachable from the Wherehouse container, not only from your browser; in Docker, use the service name (`http://paperless:8000`). **Test** under **Settings → Paperless-ngx** shows the error.

**Paperless documents don't open in the browser.** Set `PAPERLESS_PUBLIC_URL` to the address your browser uses for Paperless.

**Changes reach Warranteed only after several minutes.** Wherehouse doesn't ping Warranteed on a local address unless its host is in `WHEREHOUSE_HOOK_ALLOWED_HOSTS`. Add it, restart, and the next change is sent straight away.

**Connecting Warranteed is refused.** Both apps need versions that use PKCE when connecting (Wherehouse 0.1.0 or later). Check that `WHEREHOUSE_URL` in Warranteed is reachable from the Warranteed container.

**A reminder URL is refused.** For people who aren't admins, self-hosted services must be on a host in `WHEREHOUSE_NOTIFY_ALLOWED_HOSTS`.

**Label QR codes open the wrong address.** Labels lead to `WHEREHOUSE_APP_URL`; set it to the public URL before printing.
