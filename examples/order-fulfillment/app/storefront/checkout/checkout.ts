import { quote } from "../pricing/index";
import { Client, Checkout, Order } from "../client/index";
import { Cart } from "../cart/cart";
export const buttonLabel = "Place order";
export async function checkout(client: Client, cart: Cart, id: string): Promise<Order> {
 if (cart.items.length !== 1) throw new Error("demo supports one SKU per order");
 void quote();
 const line = cart.items[0];
 const request: Checkout = {id, sku: line.sku, quantity: line.quantity, amount: cart.total};
 return client.place(request);
}
