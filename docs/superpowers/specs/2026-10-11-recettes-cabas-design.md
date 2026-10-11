# Recettes & Cabas : récap hebdomadaire et bot Telegram

## Objectif

Chaque mercredi, recevoir sur Telegram un récap des recettes et options de la semaine sur
recettesetcabas.com, en voyant d'un coup d'œil ce qui est sélectionné et quels éléments
contiennent des ingrédients à surveiller (champignons à éviter, salades à limiter). Depuis
ce message, modifier la sélection avec des boutons, puis recevoir le panier qui sera
réellement appliqué.

Le bot **signale**, il ne décide jamais. Il ne modifie le site que sur action explicite.

## Contexte du site (constaté sur la page sauvegardée)

- Page « Modifier mes préférences » : un formulaire `POST`
  (`?Action=espaceClientAbonnement&Etape=validerPreference`) soumis par le bouton « VALIDER ».
- Recettes : 9 par semaine, en cases à cocher cachées
  `input[name="rtype_ids[]"]`. Le libellé `label[for=…]` vaut par exemple `Recette 2 - viande 1` et
  fournit le numéro du site (1 à 9). Une case cochée signifie « sélectionnée ».
- Options : environ 10 à 12, en `input[name="opt_ids[]"]`. Le libellé contient la catégorie et le
  prix, par exemple `Kit dessert à cuisiner (12.95€)`.
- Nom de l'élément : le lien `a[data-src="#recette_detail_<id>"]` qui se trouve dans le même
  `span.checkbox_formule` (`data-ref="Recette"` ou `data-ref="Option"`).
- Détail (le contenu de la modale) déjà présent dans le DOM : `#recette_detail_<id>`, avec des blocs
  `.h5` « Ingrédients du cabas » et « Ingrédients du placard », chacun suivi d'un `<p>` de texte libre.
- Semaine : `<h3>Recettes de la semaine du 19/10/2026</h3>`.
- Date limite : `Mes modifications prendront effet le <strong>Jeudi 15 Octobre 2026 à 23h00…`.
  Passé ce moment, les modifications s'appliquent à la semaine suivante, et la page affiche
  alors le catalogue suivant.
- Prix : `#js_cabas_prix` et `#js_total_prix`, remplis par le JavaScript (donc vides dans une
  page sauvegardée).
- La sélection est une **préférence persistante** : elle reste valable d'une semaine à l'autre
  jusqu'au prochain changement. C'est voulu.
- Aucune limite sur le nombre de recettes : le message « 6 recettes max » est un reliquat.

## Architecture

Le code vit dans ce repo et produit le même binaire. Il réutilise `internal/pw` (cycle de
vie Playwright, cookies, screenshot en cas d'erreur) ainsi que le packaging Nix.

```
internal/recettescabas/
  extraction de la page   page.Evaluate → Week, Deadline, Price, []Item
  marquage                mots-clés de la config → drapeaux par Item
  rendu                   texte du récap + clavier inline
  application             coche/décoche les cases, soumet, relit la page
  client Telegram         net/http : getUpdates, sendMessage, editMessageText, answerCallbackQuery
```

Deux sous-commandes Kong dans `main.go` :

- `recettes-cabas recap` : se connecte, extrait la page, envoie le récap avec son clavier,
  puis se termine. Lancée par un timer.
- `recettes-cabas bot` : boucle de long polling Telegram qui traite les boutons et les commandes.

Aucune nouvelle dépendance.

### Modèle

```go
type Item struct {
	N           int    // numéro « Recette N » pour les recettes ; 10+ pour les options, dans l'ordre de la page
	ID          string // valeur de la case à cocher (rtype_ids[] / opt_ids[])
	Name        string
	Label       string // « viande 1 », ou « Kit dessert à cuisiner (12.95€) »
	Option      bool
	Selected    bool
	Ingredients string // texte « cabas » + « placard »
}
```

La page fournit en plus : la semaine, la date limite, le nombre de personnes et le prix
(qui peut être absent).

Les options exclues par la config sont retirées **avant** la numérotation, pour que les
numéros restent ceux de la page. La numérotation est stable pendant toute la semaine.

## Configuration

Les règles métier vivent dans un fichier JSON passé par `--config`. Ce fichier est généré
par Nix (`builtins.toJSON`) et lu avec `encoding/json`. Aucune liste de mots-clés n'est
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

```
🧺 Semaine du 19/10/2026 · modifiable jusqu'au jeudi 15/10 23h00
4 pers. · 102,25 €
⚠️ Sélection : 🥗 6

✅ Sélection
2. Filet mignon aux pommes - pommes de terre (viande 1)
6. 🥗 Quiche lorraine aux poireaux - salade (protéinée) — salade
…

➕ Disponibles
1. 🍄 Gratin champignons-boulgour-mascarpone (top chrono) — champignons
3. Filet de poisson pané maison, sauce citronnée - Carottes Vichy (poisson)
…
10. Tarte fondante aux noix (Kit dessert à cuisiner (12.95€))
…
```

- Chaque drapeau est suivi du mot réellement trouvé dans les ingrédients, pour repérer les
  faux positifs.
- Une ligne d'alerte en tête liste les éléments **sélectionnés** qui portent un drapeau
  (elle est omise s'il n'y en a aucun).
- Le prix est omis s'il est absent.
- Les ingrédients complets ne figurent pas dans le récap : on les obtient avec `/detail N`.

### Clavier inline

Un bouton par élément, 5 par ligne, puis une ligne d'actions :

```
[ 1 ] [ ✅2 ] [ 3 ] [ ✅4 ] [ 5 ]
[ ✅6 ] [ 7 ] [ ✅8 ] [ ✅9 ] [ 10 ]
…
[ ✔️ Valider (2) ] [ ✖️ Annuler ]
```

États d'un bouton (le libellé encode l'état ; `callback_data` vaut `t:<N>`) :

| Libellé | Sens | Appui suivant |
|---|---|---|
| `N` | non sélectionné | `➕N` (ajout prévu) |
| `➕N` | ajout prévu | `N` (ajout annulé) |
| `✅N` | sélectionné | `➖N` (retrait prévu) |
| `➖N` | retrait prévu | `✅N` (retrait annulé) |

- Un appui sur un numéro **ne touche pas au site** : seul le clavier change
  (`editMessageReplyMarkup`).
- `✖️ Annuler` (`x`) remet tous les boutons à leur état d'origine.
- `✔️ Valider (k)` (`v`) n'apparaît que si k > 0.
- On ne peut ajouter qu'un élément non sélectionné et ne retirer qu'un élément sélectionné :
  c'est garanti par construction.
- **Pas d'état côté serveur** : les changements en attente se lisent dans le clavier du
  message, et la semaine dans son texte. Telegram renvoie les deux avec chaque appui.

### Commandes

- `/recap` : envoie un nouveau récap avec un clavier frais.
- `/detail N` : affiche le nom, le libellé, les drapeaux et le texte complet des ingrédients de
  l'élément N.
- Tout autre message : une courte aide.

## Application (« Valider »)

1. `answerCallbackQuery` immédiat, puis le message passe à « ⏳ application… ».
2. Playwright se connecte, charge la page et en extrait la semaine.
3. **Garde-fou de semaine** : si la semaine de la page diffère de celle du message, le bot
   n'applique rien et répond « la semaine a changé, utilise /recap ».
4. Pour chaque changement, le bot applique l'**état voulu** avec
   `SetChecked(true/false)` sur l'input. L'opération est idempotente : un Valider traité
   deux fois ne fait pas de dégât.
5. Il soumet le formulaire via le bouton « VALIDER ». Le mécanisme exact (clic contre
   `requestSubmit`, et le fait que le JS du site réagisse ou non à `SetChecked`) est à vérifier
   contre le vrai site au moment de l'implémentation.
6. Il recharge la page, l'extrait, et remplace le message par le nouveau récap avec un
   clavier frais. Ce récap fait office de confirmation de l'état réel.

En cas d'échec, le bot envoie « ❌ échec : <erreur> » et restaure le clavier avec les
changements en attente, pour qu'on puisse réessayer. Le screenshot `pw` est enregistré
comme d'habitude.

## Sécurité

- Seuls les updates dont le `chat_id` vaut `TELEGRAM_CHAT_ID` sont traités. Les autres sont
  ignorés, avec une ligne de log qui donne leur `chat_id` (c'est ainsi qu'on récupère le
  sien à l'installation).
- Les secrets sont fournis uniquement par l'environnement, via agenix.

## Déploiement (nixos-config, srv-1)

Nouveau fichier `hosts/srv-1/services/recettes-cabas.nix`, séparé de
`paperless-downloader.nix` puisque ce n'est pas lié à paperless :

- Fichier de config : `pkgs.writeText "recettes-cabas.json" (builtins.toJSON { … })`.
- Timer `recettes-cabas-recap` : `OnCalendar = "Wed 18:00"`, `Persistent = true`, healthchecks
  (start / success / failure), comme les downloaders existants.
- Service `recettes-cabas-bot` : `Restart = always`.
- Les deux : `DynamicUser = true`, `StateDirectory = "recettes-cabas"`, `WorkingDirectory` et `HOME`
  pointés sur ce dossier (cookies, screenshots), `--headless`.
- Secret agenix `recettes-cabas-env` contenant `RC_USERNAME`, `RC_PASSWORD`, `TELEGRAM_TOKEN`
  et `TELEGRAM_CHAT_ID`.

Limite assumée : si le timer tourne pendant une validation depuis le bot, deux navigateurs
partagent `cookies.json`. Le pire cas est un échec qu'il suffit de réessayer. On
sérialisera si ça arrive un jour.

## Tests

Une fixture `internal/recettescabas/testdata/recettes.html` est tirée de la page sauvegardée,
**réduite au bloc formulaire recettes + options**, et vérifiée exempte de toute donnée
personnelle (nom, adresse, téléphone, bloc livraison) avant d'être committée, car le repo
est public. Une config d'exemple vit dans `testdata/config.json`.

Les tests automatisés couvrent :

- **Extraction** (Playwright sur `file://`) : 9 recettes numérotées 1 à 9, les options sans les
  vins, l'état sélectionné (2, 4, 8 et 9 dans la fixture), les ingrédients non vides, la
  semaine et la date limite.
- **Correspondance** : pluriels (`champignon` trouve « champignons »), accents, mot entier
  (`vin` ne trouve pas « vinaigre »), mots-clés de plusieurs mots.
- **Portée** : drapeaux sur les ingrédients seulement (le titre est ignoré) ; exclusions sur
  les options seulement (une recette contenant « rosé » reste présente).
- **Config** : sans règle 🍄 dans la config, la recette 1 n'a pas de drapeau (rien n'est codé en dur).
- **Clavier** : l'aller-retour entre un clavier et les changements en attente, les bascules
  (sélectionné → retrait prévu → sélectionné), et Annuler.

Vérifications manuelles contre le vrai site et un vrai bot : la soumission du formulaire,
le client Telegram, et le garde-fou de semaine.

## Hors périmètre

- Changer le nombre de personnes, la livraison ou toute autre préférence.
- Une analyse des ingrédients par LLM (à envisager seulement si les mots-clés produisent trop
  de faux positifs).
- Une syntaxe en texte libre (« ajoute 3, retire 4 »).
- Des rappels le jeudi.
