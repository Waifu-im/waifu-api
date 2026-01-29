using Microsoft.EntityFrameworkCore.Migrations;

#nullable disable

namespace WaifuApi.Infrastructure.Migrations
{
    /// <inheritdoc />
    public partial class TagSlugDefaultAndOnDeleteNull : Migration
    {
        /// <inheritdoc />
        protected override void Up(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.DropForeignKey(
                name: "FK_Images_Artists_ArtistId",
                table: "Images");

            migrationBuilder.AlterColumn<string>(
                name: "Slug",
                table: "Tags",
                type: "text",
                nullable: false,
                oldClrType: typeof(string),
                oldType: "text",
                oldDefaultValue: "");

            migrationBuilder.AddForeignKey(
                name: "FK_Images_Artists_ArtistId",
                table: "Images",
                column: "ArtistId",
                principalTable: "Artists",
                principalColumn: "Id",
                onDelete: ReferentialAction.SetNull);
        }

        /// <inheritdoc />
        protected override void Down(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.DropForeignKey(
                name: "FK_Images_Artists_ArtistId",
                table: "Images");

            migrationBuilder.AlterColumn<string>(
                name: "Slug",
                table: "Tags",
                type: "text",
                nullable: false,
                defaultValue: "",
                oldClrType: typeof(string),
                oldType: "text");

            migrationBuilder.AddForeignKey(
                name: "FK_Images_Artists_ArtistId",
                table: "Images",
                column: "ArtistId",
                principalTable: "Artists",
                principalColumn: "Id");
        }
    }
}
