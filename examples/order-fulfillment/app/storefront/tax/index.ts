import type { Quote as PricingQuote } from "../pricing/index";
import { quote as next } from "../routing/index";
export interface Quote { amount: number }
export function quote(): number { return next() + 1; }
export type UpstreamQuote = PricingQuote;
