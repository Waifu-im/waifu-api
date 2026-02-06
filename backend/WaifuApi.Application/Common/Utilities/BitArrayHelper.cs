using System;
using System.Collections;

namespace WaifuApi.Application.Common.Utilities;

public static class BitArrayHelper
{
    public static string ToHex(BitArray bits)
    {
        // Convertit un BitArray en chaîne hexadécimale (ex: "4a2b...")
        var bytes = new byte[(bits.Length + 7) / 8];
        bits.CopyTo(bytes, 0);
        return Convert.ToHexString(bytes).ToLowerInvariant();
    }

    public static BitArray FromHex(string hex)
    {
        // Convertit une chaîne hexadécimale en BitArray
        var bytes = Convert.FromHexString(hex);
        return new BitArray(bytes);
    }
}