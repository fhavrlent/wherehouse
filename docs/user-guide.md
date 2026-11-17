# User guide

How to use Wherehouse day to day. Setting up the server and its connections to Paperless-ngx, Warranteed and notification services is covered in the [README](../README.md) and the [integration guide](integrations.md).

- [Signing in](#signing-in)
- [Places and things](#places-and-things)
- [Adding things](#adding-things)
- [Moving things](#moving-things)
- [Finding things](#finding-things)
- [Labels](#labels)
- [Details of a thing](#details-of-a-thing)
- [Photos, documents and links](#photos-documents-and-links)
- [Maintenance and reminders](#maintenance-and-reminders)
- [Value and insurance](#value-and-insurance)
- [Tags, kinds and custom fields](#tags-kinds-and-custom-fields)
- [Copies and templates](#copies-and-templates)
- [Archiving, selling and deleting](#archiving-selling-and-deleting)
- [Households](#households)
- [Export and backups](#export-and-backups)
- [Other apps](#other-apps)
- [Settings](#settings)

## Signing in

Click **Continue with …** and sign in with your identity provider. Wherehouse has no passwords of its own; your name, email and picture come from the provider.

## Places and things

Everything in Wherehouse is a thing, and any thing can hold others: a house holds rooms, a room a shelf, the shelf a box, the box a drill. How deep it goes is up to you.

Some kinds of things are **places**: building, room, furniture and box come built in, and you can mark your own kinds as places too. Places are shown as cards, and they're what you're offered when you put something somewhere. A thing that isn't a place can still hold things, such as a toolbox or a camera bag.

The **Things** page shows your top-level places, maintenance that's due, and what changed recently. Opening a place shows the places inside it as cards and its things as a list, with the path to it at the top.

## Adding things

**Add** in the top bar, or the round **+** on a phone, opens a short form: a photo and a name are enough. On a phone the camera opens straight away; **Choose photos** picks them from the gallery instead.

- **Where** is the place it goes in. Type to find a place, or to add a new one on the spot. Adding from a place's page puts the thing in that place.
- **More** shows the kind, quantity, tags and other details, and **From template** starts from a [template](#copies-and-templates).
- **Add and next** saves and keeps the form open with the same place, for filling a box or shelf in one go.

**Add place** on the Things page, or **Add a place inside** on a place's page, adds a place.

Things added without a place are listed under **Not put anywhere** on the Things page until you move them.

## Moving things

**Move** on a thing's page puts it, with everything in it, into another place, or into nothing. On a computer, you can also drag things onto a place card.

## Finding things

Type in the search box at the top (or press <kbd>/</kbd>). Search finds things as you type, by any word of their name, manufacturer, model, serial number, notes, tags, custom fields, label code, a kind you added, or the places they're in, so `garage drill` finds a drill anywhere in the garage. Diacritics and letter case don't matter. **Archived** searches things you've archived.

Clicking a tag shows every thing with it.

## Labels

QR labels make boxes and shelves easy to look into: scanning one opens its page.

- On a thing's page, choose **Label** to give it a label and add it to the print list. A place's menu has **Labels for everything inside**.
- **Labels** in the top bar is the print list. Pick the label sheet (common A4 and Letter sheets, or a label printer with 62 × 29 mm or 90 × 29 mm labels), set **Skip labels** to use a partly used sheet, and print at 100 % with no margins.
- **Blank labels** can be printed ahead. Stick one on, scan it, and choose **Put it on a place** or **Put it on a new thing**. To put one on a thing you already have, open the thing and choose **Label → Use a printed label**.

Scan labels with the scan button in the top bar, or with a phone's own camera app. A label of another of your households offers to switch to it.

## Details of a thing

**Edit** on a thing's page changes:

- the name, kind, quantity and what it's **inside**;
- the manufacturer, model and serial number, a description and notes;
- **Tags** (type and press Enter) and any [custom fields](#tags-kinds-and-custom-fields);
- **Purchase, warranty and insurance**: when and where it was bought, the price of one piece and its currency, the warranty end date or a lifetime warranty, warranty details, and whether it's insured;
- **Sold or given away**: when, for how much, to whom, and notes on the sale.

A thing linked to a warranty in Warranteed shows its purchase and warranty details read-only, as Warranteed keeps them; change them there.

## Photos, documents and links

- **Photos**: add as many as you like. The first one is the cover, shown in lists; any photo's menu can **Make cover**.
- **Documents**: **Upload** manuals, receipts, warranty cards and other files, or, with Paperless-ngx connected, **From Paperless** to link documents that are already there. A linked document opens for everyone in the household through the connection of whoever linked it. Removing it only unlinks it; the document stays in Paperless.
- **Links**: the shop's page, a video, the manual online.

## Maintenance and reminders

Each thing has a **Maintenance** log: services, repairs and what's due next. An entry has what was done, when it was done and when it's next due (at least one of the two dates), and optionally a cost and notes.

**Mark done** on a due entry logs it as done and asks when it's due again: in a few months, or not again. What's due soon or overdue is shown on the Things page.

To get a message when maintenance is due, add notification URLs under **Settings → Reminders**; see the [integration guide](integrations.md#reminders).

## Value and insurance

**Value** totals what your things cost (price × quantity) by place, kind and tag, and how much of it is insured. Sold and archived things don't count. Pick a place at the top to see the value of what's inside it. Things whose warranty ends within 90 days are listed below.

**Insurance list** is a printable list of insured things with where they are and what they cost, as of today, for your insurer or for your records.

## Tags, kinds and custom fields

- **Tags** are free-form. Clicking one lists its things; there you can rename it, change its colour, or delete it. Giving a tag the name of another merges the two.
- **Kinds** say what things are. Add your own under **Settings → Kinds** with an icon from about 140, or upload one, and say whether they're places.
- **Custom fields** under **Settings → Custom fields** are extra details every thing can have: text, a number, a date, a link, or yes/no.

## Copies and templates

From a thing's **⋯** menu:

- **Copy** makes another one beside it, with its photos.
- **Save as template** keeps it as a starting point, with its kind, details, price, tags and photos. Pick it with **From template** when adding a thing. Templates are listed, and deleted, under **Settings → Templates**.

## Archiving, selling and deleting

- **Archive** hides a thing from places and lists but keeps it, for things you got rid of but want to remember. Archived things are listed under **Archived** on the Things page and can be brought back.
- **Sold or given away**, in **Edit**, records a sale. A sold thing stops counting toward the value.
- **Delete** removes a thing with its photos and documents for good. Only things that hold nothing can be deleted.

## Households

A household is a set of places and things shared by the people in it: everyone in a household sees and changes all of it.

- Everyone starts in a household of their own. Its owner invites others with **Settings → Household → Invite someone**; anyone who can sign in to this Wherehouse and opens the link within 7 days joins.
- You can be in several households, such as home and a cottage, or your parents' home. Switch between them next to the logo, where **New household** makes another.
- The household's name and its default currency are under **Settings → Household**.
- **Settings → Household → Delete the household** deletes it with every place, thing, photo, document and label in it, after asking. Only a household with nobody else in it can be deleted: remove the others first, or leave it to them. The last person to leave a household deletes it too.

## Export and backups

Under **Settings → Export**:

- **Spreadsheet (CSV)** has a row for every thing.
- **Everything (ZIP)** has every detail of every thing with its photos and documents. Documents in Paperless stay there.
- **Restore from a backup…** brings an Everything (ZIP) export back as a new household, on this or another Wherehouse, and leaves the households already there alone.

## Other apps

Apps such as [Warranteed](https://github.com/fhavrlent/warranteed) connect through a page where you pick the household and click **Allow**; they then keep their details in step with Wherehouse. Each connected app has a token under **Settings → API tokens**, where deleting it disconnects the app. You can also make a token yourself there for your own scripts; [docs/api.md](api.md) describes the API.

## Settings

| Section | What's there |
| --- | --- |
| **Display** | Language and theme (light, dark or as the system) |
| **Household** | Name, currency, people and invitations |
| **Kinds** | What things can be, their icons, and which are places |
| **Custom fields** | Extra details every thing can have |
| **Templates** | Things to start new ones from |
| **Reminders** | Notification URLs and how many days ahead to remind you |
| **Paperless-ngx** | Your Paperless API token, and links from documents back to their things |
| **API tokens** | Tokens of connected apps and your own |
| **Export** | Spreadsheet and ZIP export, and restoring a backup |

Changes in Settings save automatically.
