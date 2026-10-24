import { quote as next } from "../discounts/index";
export interface Quote { amount: number }
export function quote(): number { return next() + 1; }
