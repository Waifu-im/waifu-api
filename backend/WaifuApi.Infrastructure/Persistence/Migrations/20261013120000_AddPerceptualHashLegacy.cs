using System;
using System.Collections;
using Microsoft.EntityFrameworkCore.Migrations;

#nullable disable

namespace WaifuApi.Infrastructure.Persistence.Migrations
{
    /// <inheritdoc />
    public partial class AddPerceptualHashLegacy : Migration
    {
        /// <inheritdoc />
        protected override void Up(MigrationBuilder migrationBuilder)
        {
            // Adds a column rather than changing PerceptualHash, so the C#-produced
            // fingerprints survive the Go backfill. It is nullable because rows uploaded
            // after the backfill will never have a legacy value.
            migrationBuilder.AddColumn<BitArray>(
                name: "PerceptualHashLegacy",
                table: "Images",
                type: "bit(64)",
                nullable: true);
        }

        /// <inheritdoc />
        protected override void Down(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.DropColumn(
                name: "PerceptualHashLegacy",
                table: "Images");
        }
    }
}