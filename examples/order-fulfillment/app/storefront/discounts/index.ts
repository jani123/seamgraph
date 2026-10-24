import { quote as next } from "../tax/index";
export interface Quote { amount: number }
export function quote(): number { return next() + 1; }
