# api.fireflyzero.com

The source code for [api.fireflyzero.com](https://api.fireflyzero.com), the REST API powering [shop.fireflyzero.com](https://shop.fireflyzero.com). Written in Go, uses Supabase for auth and Stripe for payments.

## Stripe metadata

To avoid running a separate database, we store all we can in `Product.metadata` in Stripe. Currently, the custom fields are:

* `slug`: the human-readable (but not visible in UI) unique ID of the product. Used to detect `donations` and to reference products in bundles.
* `products`: comma-separated list of product slugs of items in a bundle. A single slug can appear several times. The products will be shown in the order as they are defined in the list.
* `position`: product position in the list of products.
* `out-of-stock`: if `true`, the product is marked as "out of stock" and cannot be purchased.
* `hidden`: if `true`, the item cannot be purchased on its own but can be part of a bundle.
* `badge`: a short message to display as a badge next to the item in the UI. Usually something like "limited edition" or "early bird".
* `badge-icon`: [font-awesome](https://fontawesome.com/search?ip=classic&s=solid&ic=free-collection) icon to display with the badge.
