# Recettes & Cabas : récap hebdomadaire et bot Telegram

> **Retiré le 2026-10-14.** Le bot Telegram et le récap du mercredi ont été supprimés
> (commit `ad717c5`) : le panier se gère désormais dans Home Assistant, voir
> `2026-10-11-recettes-cabas-home-assistant-design.md`. Ce document reste comme historique.
> Ce qu'il décrit de l'extraction du site, des drapeaux d'ingrédients, des options exclues
> et de l'application de la sélection est toujours en service, par le pont avec Home Assistant.

## Objectif

Chaque mercredi, recevoir sur Telegram un récap des recettes et options de la semaine sur
recettesetcabas.com, en voyant d'un coup d'œil ce qui est sélectionné et quels éléments
contiennent des ingrédients à surveiller (champignons à éviter, salades à limiter). Depuis
ce message, modifier la sélection avec des boutons, puis recevoir le panier qui sera
réellement appliqué.

Le bot **signale**, il ne décide jamais. Il ne modifie le site que sur action explicite.

## Contexte du site (constaté sur la page sauvegardée puis sur le site réel)

- Connexion : `https://www.recettesetcabas.com/espace-client/connexion`, champs `#cli_email`,
  `#cli_mdp` et bouton `#button-connexion-1`. Les « placeholders » visibles sont des `<label>`, et
  la page contient une copie cachée des deux champs (d'où `:visible`). Un bandeau cookies se
  ferme avec « Continuer sans accepter ».
- Page « Modifier mes préférences » :
  `https://www.recettesetcabas.com/espace-client/commandes-planifiees/modifier`. C'est un
  formulaire `POST` (`?Action=espaceClientAbonnement&Etape=validerPreference`).
- Recettes : 9 par semaine, en cases à cocher cachées `input[name="rtype_ids[]"]`. Le libellé
  `label[for=…]` vaut par exemple `Recette 2 - viande 1` (parfois avec un tiret long « – ») et
  fournit le numéro du site. **Ce numéro suit le créneau, pas la position à l'écran** : le premier
  plat affiché peut être « Recette 4 ». Une case cochée signifie « sélectionnée ».
- Options : environ 10 à 12, en `input[name="opt_ids[]"]`. Le libellé contient souvent le nom et
  le prix, par exemple `Fruits Petit Format (9.10€)`.
- Nom de l'élément : le lien `a[data-src="#recette_detail_<id>"]` dans le même
  `span.checkbox_formule` (`data-ref="Recette"` ou `data-ref="Option"`). Photo d'une recette :
  `img.recette` dans le même `li`.
- Détail (le contenu de la modale) déjà présent dans le DOM : `#recette_detail_<id>`.
  - Il contient les blocs `.h5` « Ingrédients du cabas » et « Ingrédients du placard », chacun
    suivi d'un `<p>` de texte libre.
  - Le texte générique du site (« Nos recettes sont fournies… », « Les recettes qui
    comportent… ») est parfois collé aux ingrédients. Pour les options, il en est le seul
    contenu.
  - Le contenu d'une option est dans sa description (`header p`) : du texte en gras après un
    « : » (« La sélection de la semaine : **Poires et raisins** », « Un assortiment de 3 pâtes à
    tartes : **2 pâtes brisées et 1 pâte sablée** »).
- Semaine : `<h3>Recettes de la semaine du 19/10/2026</h3>`.
- Date limite : `Mes modifications prendront effet le <strong>Jeudi 15 Octobre 2026 à 23h00…`.
  Passé ce moment, les modifications s'appliquent à la semaine suivante, et la page affiche
  alors le catalogue suivant.
- Prix : `#js_total_prix` (livraison comprise), rempli par une requête JavaScript après le
  chargement.
- La sélection est une **préférence persistante** : elle reste valable d'une semaine à l'autre
  jusqu'au prochain changement. C'est voulu.
- Aucune limite sur le nombre de recettes : le message « 6 recettes max » est un reliquat.

## Architecture

Le code vit dans ce repo et produit le même binaire. Il réutilise `internal/pw` (cycle de
vie Playwright, cookies, screenshot en cas d'erreur) ainsi que le packaging Nix. Il utilise
`encoding/json/v2`, donc Go 1.27, fourni au flake par nixos-26.05.

```
internal/recettescabas/
  page.go       page.Evaluate → Page (semaine, date limite, personnes, total, []Item)
  config.go     règles métier (JSON)
  match.go      correspondance des mots-clés
  recap.go      exclusions + drapeaux (Annotate), texte du récap et du détail (HTML)
  keyboard.go   claviers inline (récap, choix de /detail)
  telegram.go   client net/http : getUpdates, sendMessage, sendMediaGroup, editMessageText,
                editMessageReplyMarkup, answerCallbackQuery, sendChatAction, setMyCommands
  bot.go        Service : récap, commandes, boutons
  prefs.go      préférence d'affichage (prefs.json)
  site.go       connexion, lecture, application des changements via Playwright
```

Deux sous-commandes Kong dans `main.go` :

- `recettes-cabas recap` : se connecte, extrait la page, envoie l'album puis le récap, et se
  termine. Lancée par un timer.
- `recettes-cabas bot` : boucle de long polling Telegram qui traite les boutons et les commandes.

Aucune nouvelle dépendance.

### Modèle

```go
type Item struct {
	N           int    // numéro « Recette N » pour les recettes ; 10+ pour les options, dans l'ordre de la page
	InputID     string // id DOM de la case à cocher (« rtype_23 », « opt_6 »)
	Name        string
	Label       string // « viande 1 », ou « Kit dessert à cuisiner (12.95€) »
	Option      bool
	Selected    bool
	Ingredients string // « Ingrédients du cabas », sans le texte générique
	Pantry      string // « Ingrédients du placard »
	Selection   string // contenu d'une option (fruits, fromages…), s'il y en a un
	Image       string // photo, recettes seulement
	Flags       []Hit  // drapeaux posés par Annotate
}
```

La page fournit en plus : la semaine, la date limite, le nombre de personnes et le total (qui
peut être absent si le site tarde à le calculer : le bot l'attend jusqu'à 5 s).

Les options sont numérotées 10+ dans l'ordre de la page, **puis** celles exclues par la
config sont retirées. Les numéros ne dépendent donc pas de la config, et la numérotation
est stable pendant toute la semaine (un trou peut apparaître là où était une option exclue).

La date limite est affichée telle que la page la donne (« Jeudi 15 Octobre 2026 à 23h00 »),
sans être interprétée : aucune logique n'en dépend.

## Configuration

Les règles métier vivent dans un fichier JSON passé par `--config`. Ce fichier est généré
par Nix (`builtins.toJSON`) et lu avec `encoding/json/v2`, qui rejette les champs inconnus. Aucune liste de mots-clés n'est
codée en dur.

```json
{
  "excludeOptions": ["château", "domaine", "vin", "rosé", "IGP", "AOC"],
  "flags": [
    {"emoji": "🍄", "keywords": ["champignon", "cèpe", "girolle", "shiitake", "pleurote", "morille", "trompette"]},
    {"emoji": "🥗", "keywords": ["salade", "laitue", "mâche", "roquette", "mesclun", "batavia", "sucrine", "romaine", "jeunes pousses"]}
  ]
}
```

Règles de correspondance, communes aux drapeaux et aux exclusions :

- Le texte et les mots-clés sont mis en minuscules et débarrassés de leurs accents
  (`cepe` trouve « Cèpes »).
- Un mot-clé correspond s'il forme un **mot entier**, éventuellement suivi d'un `s` ou d'un `x` :
  `champignon` trouve « champignons », `chou` trouve « choux », mais `vin` ne trouve pas
  « vinaigre ». Les mots-clés de plusieurs mots sont acceptés. Les pluriels irréguliers
  s'ajoutent à la main dans la config.
- `flags` s'applique **uniquement aux ingrédients** (cabas + placard), jamais au titre :
  une « salade de riz » dans le titre n'est pas un signal.
- `excludeOptions` s'applique **uniquement aux options** (nom et libellé). Une recette n'est jamais
  exclue (« filet mignon rosé » reste visible).

Les identifiants du site et de Telegram sont des options CLI ou des variables
d'environnement, comme pour les autres sous-commandes : `RC_USERNAME`, `RC_PASSWORD`,
`TELEGRAM_TOKEN` et `TELEGRAM_CHAT_ID`.

## Récap

Envoyé en HTML (tout ce qui vient du site est échappé), précédé d'un **album** des photos
des recettes (`sendMediaGroup`, légende « N. Nom »). Si l'album échoue, le récap part quand
même.

```
🧺 Semaine du 19/10/2026                       ← en gras
Modifiable jusqu'au Jeudi 15 Octobre 2026 à 23h00
Panier à cuisiner 4 pers. · 112,25 €
⚠️ Sélection : 🍄 9

✅ Recettes sélectionnées                      ← en gras
1. Wok de légumes, nouilles soba (végétarienne Tradi)
9. 🍄 Cuisses de poulet à l'italienne - Pâtes (viande 2) — champignons (en gras)

➕ Recettes disponibles
4. 🍄 Gratin champignons-boulgour-mascarpone (top chrono) — champignons
…

➕ Options disponibles
13. Fruits Petit Format (9.10€)
    ↳ Poires et raisins
…
```

- Quatre sections (recettes ou options, sélectionnées ou disponibles). Une section vide est
  omise.
- Chaque drapeau est suivi du mot réellement trouvé, en gras, pour repérer les faux positifs.
- Une ligne d'alerte en tête liste les éléments **sélectionnés** qui portent un drapeau
  (elle est omise s'il n'y en a aucun).
- Quand le libellé d'une option répète son nom, seul le reste est affiché (« (9.10€) »).
- Le contenu d'une option est affiché sous elle (`↳`).
- Les ingrédients complets ne figurent pas dans le récap : on les obtient avec `/detail`.

### Clavier inline

Deux affichages, au choix via `/affichage` (titres par défaut) :

```
Titres (un bouton par ligne)            Numéros
[ 🍽️ Recettes                    ]      [ 🍽️ Recettes ]
[ ✅ 1. Wok de légumes, nouilles soba ]  [ ✅ 1 ][ 2 ][ 3 ][ ➕ 4🍄 ][ 5 ]
[ ➕ 4. 🍄 Gratin champignons-…    ]      [ 6🥗 ][ 7🥗 ][ ✅ 8 ][ ➖ 9🍄 ]
[ ➖ 9. 🍄 Cuisses de poulet…      ]      [ 🧺 Options ]
[ 🧺 Options                     ]      [ 10 ][ 11 ][ 12 ][ 13 ][ 14 ]
[ 13. Fruits Petit Format        ]      …
[ ✔️ Valider (3) ][ ✖️ Annuler ]         [ ✔️ Valider (3) ][ ✖️ Annuler ]
[ 🌐 Ouvrir sur le site          ]      [ 🌐 Ouvrir sur le site ]
```

- Le titre est le nom coupé au premier « - » (la garniture). Les drapeaux suivent le numéro.
- « 🍽️ Recettes » et « 🧺 Options » sont des boutons inactifs (`-`), car Telegram n'a pas
  d'en-tête de section.
- « 🌐 Ouvrir sur le site » est un bouton-lien vers la page des préférences.

États d'un bouton (le préfixe encode l'état ; `callback_data` vaut `t:<N>`) :

| Préfixe | Sens | Appui suivant |
|---|---|---|
| aucun | non sélectionné | `➕` (ajout prévu) |
| `➕` | ajout prévu | aucun (ajout annulé) |
| `✅` | sélectionné | `➖` (retrait prévu) |
| `➖` | retrait prévu | `✅` (retrait annulé) |

- Un appui sur un élément **ne touche pas au site** : seul le clavier change
  (`editMessageReplyMarkup`). Le texte n'est jamais réécrit lors d'un appui, car Telegram ne
  renvoie au bot que la version sans mise en forme.
- `✖️ Annuler` (`x`) remet tous les boutons à leur état d'origine.
- `✔️ Valider (k)` (`v`) n'apparaît que si k > 0.
- On ne peut ajouter qu'un élément non sélectionné et ne retirer qu'un élément sélectionné :
  c'est garanti par construction.
- **Pas d'état côté serveur pour la sélection** : les changements en attente se lisent dans
  le clavier du message (y compris son affichage), et la semaine dans son texte. Telegram
  renvoie les deux avec chaque appui.

### Commandes

Enregistrées au démarrage du bot dans le menu `/` (`setMyCommands`) :

- `/recap` : envoie l'album et un nouveau récap avec un clavier frais.
- `/detail N` : affiche la ligne de l'élément puis deux blocs, « Ingrédients du cabas » et
  « Ingrédients du placard ». Sans numéro (le menu `/` envoie la commande aussitôt), répond
  « Quel élément ? » avec une grille de boutons (`d:<N>`).
- `/affichage` : boutons « 🔢 Numéros » / « 🔤 Titres » (`s:n` / `s:t`). Le choix est enregistré
  dans `prefs.json`, dans le répertoire de travail partagé par le bot et le récap du
  mercredi. Un fichier absent donne les titres.
- Tout autre message : une courte aide.

Pendant qu'il lit ou modifie le site (`/recap`, `/detail`, « Valider »), le bot affiche
« en train d'écrire… » (`sendChatAction`), renvoyé toutes les 4 s.

## Application (« Valider »)

1. `answerCallbackQuery` immédiat, puis les boutons sont remplacés par « ⏳ application… ».
2. Playwright se connecte, charge la page et en extrait la semaine.
3. **Garde-fou de semaine** : si la semaine de la page diffère de celle du message, le bot
   n'applique rien, retire les boutons et envoie « la semaine a changé, utilise /recap ».
4. Pour chaque changement, le bot écrit l'**état voulu** (`input.checked = true/false`). Les
   inputs sont cachés (`display:none`), donc `SetChecked` de Playwright ne peut pas les
   cocher. L'opération est idempotente : un Valider traité deux fois ne fait pas de dégât.
5. Il soumet le formulaire par un `form.submit()` natif, qui contourne le JavaScript du site.
   Tous les autres champs (personnes, robot, livraison) partent tels quels. Vérifié sur le
   site réel.
6. Il recharge la page, vérifie que chaque changement a bien été enregistré, et remplace le
   message par le nouveau récap avec un clavier frais.

En cas d'échec, le bot restaure le clavier avec les changements en attente, pour qu'on
puisse réessayer. Il envoie aussi « ❌ échec : <erreur> », limité à la première ligne de
l'erreur et à 300 caractères. Le screenshot `pw` est enregistré comme d'habitude.

## Sécurité

- Seuls les updates dont le `chat_id` vaut `TELEGRAM_CHAT_ID` sont traités. Les autres sont
  ignorés, avec une ligne de log qui donne leur `chat_id` (c'est ainsi qu'on récupère le
  sien à l'installation).
- Les secrets sont fournis uniquement par l'environnement : agenix sur srv-1, et en local
  `mise.local.toml` chiffré avec git-crypt (voir le README).
- Ni les identifiants ni le token n'apparaissent dans les erreurs :
  - une saisie Playwright ratée renvoie une erreur générique, car son journal d'appels
    reprend la valeur tapée ;
  - les `*url.Error`, dont l'URL contient le token, sont débarrassées de cette URL.

## Déploiement (nixos-config, srv-1)

Nouveau fichier `hosts/srv-1/services/recettes-cabas.nix`, séparé de
`paperless-downloader.nix` puisque ce n'est pas lié à paperless :

- Fichier de config : `pkgs.writeText "recettes-cabas.json" (builtins.toJSON { … })`.
- Timer `recettes-cabas-recap` : `OnCalendar = "Wed 18:00"`, `Persistent = true`, healthchecks
  (start / success / failure), comme les downloaders existants.
- Service `recettes-cabas-bot` : `Restart = always`.
- Les deux : même utilisateur dynamique (`User` + `DynamicUser = true`),
  `StateDirectory = "recettes-cabas"`, `WorkingDirectory` et `HOME` pointés sur ce dossier
  (cookies, `prefs.json`, screenshots), `--headless`.
- Secret agenix `recettes-cabas-env` contenant `RC_USERNAME`, `RC_PASSWORD`, `TELEGRAM_TOKEN`
  et `TELEGRAM_CHAT_ID`.
- Un seul bot par token : aucun autre processus `bot` ne doit tourner en même temps (Telegram
  répond `409 Conflict`).

Limite assumée : si le timer tourne pendant une validation depuis le bot, deux navigateurs
partagent `cookies.json`. Le pire cas est un échec qu'il suffit de réessayer. On
sérialisera si ça arrive un jour.

## Tests

Une fixture `internal/recettescabas/testdata/recettes.html` est tirée de la page sauvegardée,
**réduite au bloc formulaire recettes + options**, sans images hormis les photos des
recettes. Elle a été vérifiée exempte de toute donnée personnelle (nom, adresse, téléphone,
bloc livraison) avant d'être committée, car le repo est public. Une config d'exemple vit dans
`testdata/config.json`.

Les tests Playwright sont ignorés quand `PLAYWRIGHT_DRIVER_PATH` n'est pas défini (sandbox
`nix build`) ; `nix develop -c go test ./...` les lance.

Les tests automatisés couvrent :

- **Extraction** : numéros du site, état sélectionné (1, 2, 8 et 9 dans la fixture),
  ingrédients et placard séparés sans le texte générique, contenu des options, photos,
  semaine, date limite.
- **Correspondance** : pluriels, accents, mot entier (`vin` ne trouve pas « vinaigre »),
  mots-clés de plusieurs mots.
- **Portée** : drapeaux sur les ingrédients et le placard, jamais sur le titre ; exclusions sur
  les options seulement.
- **Config** : sans règle 🍄 dans la config, aucun drapeau (rien n'est codé en dur).
- **Rendu** : récap HTML échappé, sections, détail en deux blocs.
- **Claviers** : les deux affichages, les en-têtes, les bascules, Annuler, la relecture d'un
  clavier renvoyé par Telegram, la grille de `/detail`.
- **Bot** : chats étrangers ignorés, garde-fou de semaine, échec qui restaure le clavier,
  saisies invalides, `/affichage`, indicateur « en train d'écrire… ».
- **Sécurité** : pas de token ni de valeur saisie dans les erreurs.

Vérifiés contre le vrai site et un vrai bot : la connexion, la soumission du formulaire, le
récap, l'album, les boutons, `/detail` et `/affichage`.

## Hors périmètre

- Changer le nombre de personnes, la livraison ou toute autre préférence.
- Une analyse des ingrédients par LLM (à envisager seulement si les mots-clés produisent trop
  de faux positifs).
- Une syntaxe en texte libre (« ajoute 3, retire 4 »).
- Des rappels le jeudi.
