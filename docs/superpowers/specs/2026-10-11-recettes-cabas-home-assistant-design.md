# Recettes & Cabas : pont avec Home Assistant (sous-projet 1)

> **Telegram retiré le 2026-10-14** (commit `ad717c5`). Ce qui, ici, touche Telegram ne vaut
> plus : le bot, le récap du mercredi et son timer, le message « ✏️ » après `/apply`, l'usage
> B « remplacer Telegram pour une autre personne » (Home Assistant le remplace pour tout le
> foyer). Le pont reste tel que décrit, sauf deux points : il tourne seul, en service
> `recettes-cabas` (`recettes-cabas serve`), et ses trois réglages sont obligatoires (plus de
> mode « désactivé »). Les sous-projets 2 (tableau de bord) et 3 (notifications) sont faits
> dans Home Assistant : la section « Recettes & Cabas » du `CLAUDE.md` du dossier HA les décrit.

## Objectif

Rendre la sélection Recettes & Cabas utilisable depuis Home Assistant, pour trois usages :
- **A.** un tableau de bord pour consulter et modifier la sélection, en plus de Telegram ;
- **B.** remplacer Telegram pour une autre personne du foyer ;
- **C.** des automatisations (alerte, Assist).

Le travail est découpé en trois sous-projets. Seul le premier est traité ici :

1. **Le pont bot ↔ HA** (ce document) : le bot pousse l'état de la semaine vers HA, qui en fait
   des entités, et HA peut demander au bot de relire le site ou d'appliquer une sélection.
2. **Le tableau de bord HA** : préparation des changements (« voulu » par élément, Valider,
   Annuler) et vue des éléments avec leurs images. Config HA seulement.
3. **Notifications et automatisations HA** : notification du mercredi dans l'app compagnon,
   alerte du jeudi « 🍄 encore sélectionné », Assist. Config HA seulement.

Le pont expose ce dont 2 et 3 ont besoin, sans les implémenter.

## Contexte

- HA est un Home Assistant Green (HAOS), à `https://hass.augendre.info`, avec l'app Tailscale.
  Ses règles d'administration sont dans le `CLAUDE.md` de `~/pCloudSync/Home Assistant/`, et
  s'appliquent à tout ce qui touche HA dans ce projet.
- Le bot tourne sur srv-1 (NixOS), en service `recettes-cabas-bot`. srv-1 a Caddy avec un
  snippet `import internal`, qui limite un vhost à Tailscale et au réseau local.
- Pas de broker MQTT, et pas de nouvelle dépendance Go : tout passe par `net/http`.

## Architecture

```
Bot (srv-1) ── POST JSON ──▶ webhook HA ──▶ entités template (sensor + image)
     ▲
     └── POST /refresh, /apply ◀── rest_command HA ◀── automatisations, tableau de bord
         (vhost Caddy interne)
```

- **Bot → HA :** après **chaque** lecture du site, le bot envoie l'état complet de la semaine
  au webhook HA.
- **HA → bot :** deux endpoints HTTP du bot, appelés par des `rest_command`.
- **La fraîcheur est pilotée par HA :** une automatisation appelle `/refresh`. Le bot reste
  réactif, sans planificateur de plus.

## Côté bot (Go)

### Serveur HTTP

Le service `bot` lance, en plus de la boucle Telegram, un serveur HTTP sur l'adresse donnée par
`--api-listen` (par exemple `127.0.0.1:8789`).

- Toutes les requêtes exigent `Authorization: Bearer <RC_API_TOKEN>`. La comparaison se fait à
  temps constant (`crypto/subtle`). En cas d'absence ou d'erreur : `401`, sans corps utile.
- Les corps de requête sont limités (`http.MaxBytesReader`, 64 Kio).
- Seule la méthode `POST` est acceptée : `405` sinon.

| Endpoint | Corps | Effet | Réponses |
|---|---|---|---|
| `POST /refresh` | aucun | lit le site, pousse l'état vers HA | `204`, `502` si le site échoue |
| `POST /apply` | `{"week":"19/10/2026","desired":{"7":true,"9":false}}` | applique l'état voulu (même `Apply` que Telegram), pousse l'état, envoie un récap Telegram | `200`, `400` entrée invalide, `409` semaine changée, `502` site |

- `/apply` applique un **état voulu** et reste idempotent, comme Valider dans Telegram.
- `/apply` rejette (`400`) :
  - un `desired` vide ;
  - une clé qui n'est pas un entier ;
  - une semaine absente ou mal formée.
- Le garde-fou de semaine est le même que pour Telegram : si la semaine du site diffère de
  `week`, rien n'est appliqué et la réponse est `409`.
- En cas d'échec, le corps de la réponse est un texte court, sans identifiants : une seule
  ligne, comme le message « ❌ échec » de Telegram.

### Poussée vers HA

Une fonction `pushHA(page)` envoie `POST <RC_HA_WEBHOOK_URL>` avec le JSON ci-dessous. Elle est
appelée après toute lecture réussie du site :
- le récap (timer du mercredi, `/recap`) ;
- `/detail` ;
- Valider côté Telegram ;
- `/refresh` et `/apply`.

Un échec de poussée est **seulement logué** : il ne fait jamais échouer l'action principale.
Le timeout est de 10 s.

```json
{
  "week": "19/10/2026",
  "deadline": "Jeudi 15 Octobre 2026 à 23h00",
  "people": "Panier à cuisiner 4 pers.",
  "price": "112,25",
  "alerts": [{"n": 9, "emojis": "🍄"}],
  "items": [
    {
      "n": 4,
      "name": "Gratin champignons-boulgour-mascarpone",
      "label": "top chrono",
      "option": false,
      "selected": false,
      "flags": [{"emoji": "🍄", "word": "champignons"}],
      "selection": "",
      "ingredients": "boulgour ; champignons de Paris ; lardons ; mascarpone",
      "pantry": "Huile d'olive ; …",
      "image": "https://www.recettesetcabas.com/data/recettes/….webp"
    }
  ]
}
```

- Les éléments sont ceux du récap : après `Annotate`, donc sans les options exclues, avec leurs
  drapeaux et triés par numéro.
- `label` est le même que dans le récap Telegram (sans la répétition du nom pour les options).
- Les noms de champs sont figés : le package HA en dépend. Un test Go fixe le JSON produit pour
  une page d'exemple.
- Si un numéro dépasse 25 (le nombre d'emplacements côté HA), le bot logue un avertissement à
  chaque poussée. L'élément reste dans le JSON, mais HA ne l'affiche pas.

### Images des options

L'extraction récupère aussi la photo des options : `img` dans le `.img_container_card` du même
`li`. L'album Telegram reste **limité aux recettes**.

### Accès au site sérialisé

Telegram et HA peuvent agir en même temps. Toutes les lectures et applications du site du
processus `bot` passent par un **verrou unique**, porté par `Service`. Un seul navigateur à la
fois, et pas d'écriture concurrente de `cookies.json`.

Le timer du mercredi reste un processus séparé qui ne fait que lire : la limite déjà assumée
dans la spec principale ne change pas.

### Prévenir Telegram

Après un `/apply` réussi, le bot envoie dans Telegram l'album et un nouveau récap, précédés de la
ligne « ✏️ Sélection modifiée depuis Home Assistant ». L'ancien récap Telegram n'est pas
modifié : il peut afficher un état périmé, et le nouveau message le signale.

### Configuration

- `--api-listen` (option CLI, pas un secret).
- Variables d'environnement, fournies par le secret agenix `recettes-cabas-env` :
  - `RC_API_TOKEN` : jeton exigé par les endpoints ;
  - `RC_HA_WEBHOOK_URL` : URL complète du webhook HA (elle contient l'identifiant du webhook,
    qui est un secret de fait).
- **Si aucun des trois réglages n'est fourni, le pont est désactivé :** pas de serveur HTTP,
  pas de poussée. Le bot se comporte alors exactement comme aujourd'hui, ce qui garde le
  développement local simple. **Si un ou deux seulement sont fournis, c'est une erreur au
  démarrage** : un oubli ne doit pas passer inaperçu.

## Côté nixos-config (srv-1)

- `recettes-cabas.nix` : ajouter `--api-listen 127.0.0.1:8789` au service `bot`.
- Caddy : vhost interne `recettes-cabas.srv-1.augendre.info` avec `import internal` et
  `reverse_proxy localhost:8789`.
- Secret `recettes-cabas-env` : Gabriel y ajoute `RC_API_TOKEN` et `RC_HA_WEBHOOK_URL`
  (`agenix -e recettes-cabas-env.age`, avec le wrapper fish).

## Côté Home Assistant

Les règles du `CLAUDE.md` du dossier HA s'appliquent, notamment :
- le YAML s'écrit avec `ha_config_set_yaml` en mode aperçu ;
- les automatisations vont dans `automations.yaml` ;
- les secrets passent par `!secret` et ne sont jamais lus ;
- la mémoire est écrite dans ce dossier.

### `packages/recettes_cabas.yaml` (écrit via `ha_config_set_yaml`)

- **`template:`** : un bloc déclenché par un webhook, avec
  `webhook_id: !secret recettes_cabas_webhook_id` et `local_only: false` (voir plus bas).
  - `sensor.recettes_cabas` (global) :
    - état : la semaine ;
    - attributs : `deadline`, `people`, `price`, `alerts`, et `numbers` (la liste des numéros
      présents cette semaine, pour le tableau de bord). `price` est suffixé (« 112,25 € ») :
      HA lit le résultat d'un gabarit comme un littéral Python, et « 112,25 » seul deviendrait
      le tuple `(112, 25)`.
  - Pour N de 1 à 25 :
    - `sensor.recettes_cabas_N` :
      - état : le nom ;
      - attributs : `label`, `option`, `selected`, `flags`, `emojis`, `words`, `selection`,
        `ingredients`, `pantry` ;
      - **indisponible** si l'élément N est absent du dernier envoi.
    - `image.recettes_cabas_N` : `url` reprend l'image de l'élément N, indisponible s'il est
      absent.
  - `unique_id` stables (`recettes_cabas_N`, `recettes_cabas_N_image`), pour que les entités
    soient modifiables dans l'interface.
  - Le bloc est généré une fois (25 emplacements identiques au numéro près) et écrit tel quel.
    Une variable de bloc indexe les éléments par numéro
    (`items: "{{ dict(zip(trigger.json['items'] | map(attribute='n'), trigger.json['items'])) }}"`),
    et chaque emplacement lit `items.get(N, {}).get(...)` : sans avertissement de variable
    indéfinie quand l'élément est absent.
- **`recorder:`** : exclusion de `sensor.recettes_cabas*` et `image.recettes_cabas*`. Seul
  l'état courant compte, et les attributs (ingrédients) sont gros.

### `rest_command` (collées par Gabriel)

`ha_config_set_yaml` n'accepte pas la clé `rest_command`. Gabriel colle donc ce bloc dans le
même package via Studio Code Server, puis redémarre HA (`rest_command` ne se recharge pas à
chaud) :

```yaml
rest_command:
  recettes_cabas_refresh:
    url: https://recettes-cabas.srv-1.augendre.info/refresh
    method: post
    headers:
      authorization: !secret recettes_cabas_api_authorization   # "Bearer <RC_API_TOKEN>"
    timeout: 60
  recettes_cabas_apply:
    url: https://recettes-cabas.srv-1.augendre.info/apply
    method: post
    headers:
      authorization: !secret recettes_cabas_api_authorization
    content_type: application/json
    payload: '{"week": {{ week | to_json }}, "desired": {{ desired | to_json }}}'
    timeout: 120
```

Les secrets à ajouter dans `secrets.yaml` (par Gabriel) sont `recettes_cabas_webhook_id` et
`recettes_cabas_api_authorization`.

### Automatisation de fraîcheur (via `ha_config_set_automation`)

« Recettes & Cabas – rafraîchir » appelle `rest_command.recettes_cabas_refresh` chaque jour à
7h00, puis le vendredi à 0h15, juste après la date limite du jeudi 23h. HA voit ainsi la
nouvelle semaine sans attendre le récap du mercredi.

### Documentation

À la fin, une section « Recettes & Cabas » dans le `CLAUDE.md` du dossier HA. Elle décrit les
entités, le webhook, les `rest_command`, l'automatisation, la répartition entre ce qui est
écrit par le MCP et ce qui est collé à la main, et les deux secrets.

## Points à vérifier pendant l'implémentation

- **`local_only` du webhook : tranché, `false`.** HA juge `request.remote` par
  `network_util.is_local` (loopback, RFC 1918, `fd00::/8`, link-local) : les IP Tailscale ne
  sont pas locales, et un appel refusé répond quand même `200`. L'identifiant du webhook est
  donc aléatoire, 48 caractères hexadécimaux. Consigné dans la doc HA.
- **Images `.webp`.** Vérifier qu'une entité `image` template sert bien ces images dans
  l'interface et dans l'app mobile.
- **Taille.** Les attributs d'un emplacement restent petits (moins de 2 Ko). À confirmer sur
  les données réelles.

## Erreurs et cas limites

- Poussée HA en échec : log, l'action continue.
- `/apply` pendant qu'une action Telegram tourne : attente du verrou. Le `timeout` de 120 s de
  la `rest_command` couvre ce cas.
- Semaine changée (`409`) : HA reçoit l'erreur. Le tableau de bord (sous-projet 2) devra
  proposer un rafraîchissement.
- Élément absent de la semaine : les entités de l'emplacement sont indisponibles.
- Plus de 25 éléments : avertissement dans les logs du bot.
- Jeton invalide : `401`, sans détail.

## Tests

- **HTTP** (`httptest`, avec `Fetch` et `Apply` factices) :
  - `401` sans jeton ou avec un mauvais jeton ; `405` sur `GET` ;
  - `/refresh` donne `204` et pousse ;
  - `/apply` : entrée invalide (`400`), semaine changée (`409`), succès (`200`, poussée,
    message Telegram « ✏️ ») ;
  - `502` sur une erreur de site, sans détail sensible.
- **Poussée** : le JSON exact pour une page d'exemple (test de référence), et un échec du
  webhook qui ne fait pas échouer l'action.
- **Verrou** : deux actions concurrentes ne lancent jamais deux lectures du site en même temps.
- **Config** : un pont partiellement configuré provoque une erreur au démarrage ; non
  configuré, il est désactivé.
- **Extraction** : image d'une option dans la fixture ; l'album Telegram reste limité aux
  recettes.
- **HA** : après déploiement, un `/refresh` crée les entités. Vérifier un emplacement de
  recette, une option avec son image, un emplacement vide (indisponible), puis un `/apply`
  depuis l'outil de développement de HA (Actions), suivi du récap « ✏️ » dans Telegram.

## Hors périmètre (sous-projets 2 et 3)

- Les aides « voulu » par élément, les scripts Valider et Annuler, le tableau de bord.
- La notification du mercredi dans l'app compagnon, l'alerte du jeudi, Assist.
- Le partage par groupe Telegram (`2026-10-11-recettes-cabas-group-design.md`).
