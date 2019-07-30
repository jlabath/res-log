module HistoryView exposing (Log, Model, Msg(..), add, empty, renderModel, view)

import Html exposing (Html, div, li, text, ul)
import Html.Attributes exposing (id)
import Html.Events exposing (on)
import Json.Decode as Decode


type alias Model =
    { resId : String
    , resType : String
    , resTypeLabel : String
    }


type Msg
    = Clicked Model


type alias Log =
    { records : List Model }


empty : Log
empty =
    Log []


add : Model -> Log -> Log
add m log =
    let
        newrecords =
            List.filter (\x -> x /= m) log.records
    in
    { log | records = m :: newrecords }


view : Log -> Html Msg
view log =
    div [ id "history" ]
        [ ul [] <| List.map renderModel log.records
        ]


renderModel : Model -> Html Msg
renderModel m =
    li [ on "click" (Decode.succeed <| Clicked <| m) ] [ text (m.resTypeLabel ++ " " ++ m.resId) ]
