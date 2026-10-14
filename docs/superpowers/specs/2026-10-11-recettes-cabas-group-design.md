# Recettes & Cabas : partager le bot dans un groupe Telegram

État : **abandonné le 2026-10-14**, jamais implémenté. Telegram a été retiré : le panier se
partage dans Home Assistant (tableau de bord et notifications sur les deux téléphones), voir
`2026-10-11-recettes-cabas-home-assistant-design.md`.

## Besoin

Partager le bot avec une autre personne (même compte Recettes & Cabas, même panier) : tous deux
reçoivent le récap et peuvent modifier la sélection.

## Décision : un groupe Telegram, pas une conversation de plus

Le bot est ajouté à un groupe contenant les deux personnes, et `TELEGRAM_CHAT_ID` devient
l'identifiant du groupe.

- **Une seule source de vérité** : un seul récap, un seul clavier. Les changements en attente
  (`➕7`) sont visibles par tous avant que quelqu'un valide.
- **Alternative écartée : plusieurs conversations privées.** Chacun aurait son récap. Après une
  validation de l'un, le message de l'autre afficherait un état périmé. Comme « Valider »
  applique un état voulu, la validation suivante pourrait écraser la précédente sans que
  personne ne s'en aperçoive. Il faudrait diffuser chaque message et chaque mise à jour dans
  toutes les conversations : plus de code pour une expérience moins bonne.

## Changements de code

- **Suffixe des commandes** : dans un groupe, Telegram envoie `/recap@NomDuBot`, et non
  `/recap`. `Service.message` doit retirer le suffixe `@…` avant de reconnaître la commande
  (`/recap`, `/detail`, `/affichage`), y compris quand un argument suit (`/detail@NomDuBot 6`).
  Il faut un test pour chaque forme.
- **Rien d'autre** :
  - Le mode confidentialité du bot (actif par défaut) suffit : dans un groupe, il ne reçoit que
    les commandes et les appuis sur ses boutons, pas les autres messages.
  - `setMyCommands` sans portée s'applique aussi aux groupes : le menu `/` y apparaît.
  - Le filtrage par `chat_id` continue de fonctionner tel quel, avec l'identifiant du groupe.

## Mise en place

1. Créer le groupe avec les deux personnes, puis y ajouter le bot.
2. Récupérer l'identifiant du groupe : un nombre négatif (`-100…`). Avec le bot déployé, taper
   `/recap` dans le groupe ; le bot logue `ignoring update from chat <id>`
   (`journalctl -u recettes-cabas-bot` sur srv-1).
3. Mettre `TELEGRAM_CHAT_ID=<id du groupe>` dans le secret
   (`agenix -e recettes-cabas-env.age` dans `nixos-config/hosts/srv-1/age`, avec le wrapper fish
   de dotfiles), commit, push, `mise run srv-1`.
4. Mettre la même valeur dans `mise.local.toml` (git-crypt) pour les essais en local.

## Points d'attention

- **Sécurité** : tout membre du groupe peut modifier la sélection. Le groupe est contrôlé par son
  créateur, ce qui suffit. Si besoin un jour : une liste d'identifiants d'utilisateurs autorisés,
  vérifiée sur `from.id` des messages et des callbacks.
- **Migration en supergroupe** : certains changements de réglages convertissent un groupe en
  supergroupe, ce qui **change son identifiant**. Le bot ignore alors le groupe et logue le
  nouvel identifiant (`ignoring update from chat …`) ; il suffit de mettre le secret à jour.
