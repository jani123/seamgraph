import { Client, Status } from "../client/index";
export async function status(client: Client, id: string): Promise<Status> {
 return (await client.status(id)).status;
}
