import { Cart } from "../cart/cart";
import { Client, Checkout, Order } from "../client/index";
import { checkout } from "../checkout/checkout";
async function main() {
 const requests: Checkout[] = [];
 const client = new Client({post: async request => { requests.push(request); return {id: request.id, status: "pending"}; }, get: async id => ({id, status: "pending"})});
 const cart = new Cart(); cart.add({sku: "book", quantity: 2, unitPrice: 600});
 const result: Order = await checkout(client, cart, "o-1");
 if (result.id !== "o-1" || requests.length !== 1 || requests[0].amount !== 1200) throw new Error("checkout contract");
 let rejected = false;
 try { await checkout(client, new Cart(), "empty"); } catch { rejected = true; }
 if (!rejected) throw new Error("empty cart accepted");
}
void main();
