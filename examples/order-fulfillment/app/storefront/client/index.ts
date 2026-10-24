export type Status = "pending" | "fulfilled" | "cancelled";
export interface Order { readonly id: string; status: Status }
export interface Checkout { id: string; sku: string; quantity: number; amount: number }
export interface Transport { post(request: Checkout): Promise<Order>; get(id: string): Promise<Order> }
export class Client {
 constructor(private readonly transport: Transport) {}
 place(request: Checkout): Promise<Order> { return this.transport.post(request); }
 status(id: string): Promise<Order> { return this.transport.get(id); }
}
export const currency = "USD";
export enum Screen { Cart, Checkout, Status }
